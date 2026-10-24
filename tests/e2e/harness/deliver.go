package harness

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/orieken/loom/internal/factory"
	"github.com/orieken/loom/internal/intake"
	"github.com/orieken/loom/internal/planfile"
	"github.com/orieken/loom/internal/worktree"
)

// Deliver builds the fixture repository with its order committed, then runs
// the factory pass by pass, approving whatever the inbox asks for, until the
// order ships, parks, or stops making progress. Digests are checked before
// the commit is approved, while the run's worktree still exists.
func Deliver(config Config) (Report, error) {
	config = config.withDefaults()
	id, err := orderID(config.Fixture)
	if err != nil {
		return Report{}, err
	}
	if err := setUpProject(config, id); err != nil {
		return Report{}, err
	}
	report := Report{Order: id}
	report.record("the factory shipped the order", deliverOrder(config, &report))
	return report, saveReport(config, report)
}

// orderID is the order's id, which is also the file name it is committed as.
func orderID(fixture string) (string, error) {
	source, err := os.ReadFile(filepath.Join(fixture, "order.md"))
	if err != nil {
		return "", fmt.Errorf("read the fixture's order: %w", err)
	}
	document, err := intake.Parse(source)
	if err != nil {
		return "", fmt.Errorf("parse the fixture's order: %w", err)
	}
	return document.Order.ID, nil
}

// setUpProject makes the project repository from the fixture, commits the
// order to intake/ on main, and pushes main to a bare origin.
func setUpProject(config Config, id string) error {
	project := config.project()
	if err := copyTree(filepath.Join(config.Fixture, "repo"), project); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(config.Fixture, "order.md"), filepath.Join(project, factory.IntakeDir, id+".md")); err != nil {
		return err
	}
	steps := [][]string{
		{"init", "-q", "--bare", config.origin()},
		{"-C", project, "init", "-q", "-b", "main"},
		{"-C", project, "config", "user.email", "e2e@loom.invalid"},
		{"-C", project, "config", "user.name", "loom e2e harness"},
		{"-C", project, "add", "-A"},
		{"-C", project, "commit", "-qm", "fixture with order " + id},
		{"-C", project, "remote", "add", "origin", config.origin()},
		{"-C", project, "push", "-q", "-u", "origin", "main"},
	}
	for _, step := range steps {
		if _, err := run(config.Workdir, config.PassTimeout, "git", step...); err != nil {
			return err
		}
	}
	return nil
}

func deliverOrder(config Config, report *Report) error {
	for pass := 1; pass <= config.MaxPasses; pass++ {
		fmt.Fprintf(config.Output, "pass %d\n", pass)
		if err := factoryPass(config); err != nil {
			return err
		}
		done, err := approveWhatWaits(config, report)
		if done || err != nil {
			return err
		}
	}
	return fmt.Errorf("not shipped after %d passes", config.MaxPasses)
}

// factoryPass is one `loom factory run --once`, the way cron would run it.
func factoryPass(config Config) error {
	args := []string{"factory", "run", "--once", "--provider", config.Provider, "--retry-base", "0s"}
	for _, arg := range config.RunArgs {
		args = append(args, "--run-arg="+arg)
	}
	output, err := run(config.project(), config.PassTimeout, config.Loom, args...)
	fmt.Fprint(config.Output, output)
	return err
}

// approveWhatWaits approves every inbox item for the order, explicitly and
// one at a time through `loom factory approve`. It reports done once the
// order has shipped, and fails an order that parked, stopped, or waits on
// nothing while not done.
func approveWhatWaits(config Config, report *Report) (bool, error) {
	order, items, err := readInbox(config, report.Order)
	if err != nil {
		return false, err
	}
	if done, err := settled(order, items); done || err != nil {
		return done, err
	}
	for _, item := range items {
		if err := approve(config, report, order, item); err != nil {
			return false, err
		}
	}
	return false, nil
}

// settled says whether the order is finished: shipped is done; parked,
// stopped, or waiting on nothing is a failure, because another pass would
// change nothing.
func settled(order *factory.OrderState, items []factory.InboxItem) (bool, error) {
	switch {
	case order.Phase == factory.PhaseShipped:
		return true, nil
	case order.Phase == factory.PhaseParked, order.Phase == factory.PhaseStopped:
		return false, fmt.Errorf("order %s: %s", order.Phase, order.Reason)
	case len(items) == 0:
		return false, fmt.Errorf("order is %s with nothing to approve: %s", order.Phase, order.Reason)
	}
	return false, nil
}

func approve(config Config, report *Report, order *factory.OrderState, item factory.InboxItem) error {
	if item.Gate == factory.GateCommit {
		checkDigests(report, order)
	}
	output, err := run(config.project(), config.PassTimeout, config.Loom, "factory", "approve", order.ID, item.Gate)
	fmt.Fprint(config.Output, output)
	if err != nil {
		return err
	}
	report.Approved = append(report.Approved, item.Gate)
	return nil
}

// readInbox returns the order's state and what the inbox asks of a human
// for it, read the same way `loom factory inbox` reads them.
func readInbox(config Config, id string) (*factory.OrderState, []factory.InboxItem, error) {
	layout := factory.Layout{Root: config.project()}
	entries, err := factory.OpenLedger(layout.LedgerPath()).Entries()
	if err != nil {
		return nil, nil, err
	}
	orders := factory.Replay(entries)
	order, ok := orders[id]
	if !ok {
		return nil, nil, fmt.Errorf("the factory never claimed %s", id)
	}
	committed, err := factory.ReadIntake(worktree.New(layout.Root), "main", plansIn(layout.Root))
	if err != nil {
		return nil, nil, err
	}
	items, err := factory.Inbox(map[string]*factory.OrderState{id: order}, committed, openTree)
	return order, approvable(items), err
}

func openTree(dir string) factory.Tree { return worktree.New(dir) }

func approvable(items []factory.InboxItem) []factory.InboxItem {
	var asking []factory.InboxItem
	for _, item := range items {
		if item.IsApprovable() {
			asking = append(asking, item)
		}
	}
	return asking
}

func plansIn(root string) []string {
	var names []string
	for _, available := range planfile.Discover(planfile.SearchPathFrom(root)) {
		if available.Err == nil {
			names = append(names, available.Name)
		}
	}
	return names
}
