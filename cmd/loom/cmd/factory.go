package cmd

// `loom factory` — the software factory's supervisor and its controls
// (roadmap L3.65–L3.70). The supervisor reads work orders committed on a
// branch, runs each ready one through `loom run` in its own worktree, and
// records everything in .claude/factory/ledger.jsonl. Every other
// subcommand reads that ledger or appends a human's decision to it.

import (
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/orieken/loom/internal/factory"
	"github.com/orieken/loom/internal/worktree"
	"github.com/spf13/cobra"
)

type factoryFlags struct {
	once          bool
	maxConcurrent int
	interval      time.Duration
	branch        string
	provider      string
	retryBase     time.Duration
	retryMax      time.Duration
	runArgs       []string
}

var factoryArgs factoryFlags

var factoryCmd = &cobra.Command{
	Use:   "factory",
	Short: "Run work orders from intake/ unattended, and approve what they wait on",
	Args:  cobra.NoArgs,
}

var factoryRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Supervise: claim ready orders, run them, record every outcome",
	Long: `Reads intake/*.md as committed on --branch (never your working tree), and
claims every valid order with status: ready whose depends-on have shipped, by
priority, then created, then id — up to --max-concurrent at a time.

Each claim gets a git worktree on branch factory/<id> under .claude/factory/,
and a snapshot of the order the run reads as its spec, so editing the intake
file never changes a running order. The order's budget is passed to loom run
as its ceiling.

A run that halts at a gate waits in 'loom factory inbox'. A transient failure
is resumed with exponential backoff up to the order's max-attempts; anything
else is parked with its reason, and editing the order clears it. A completed
run is committed and pushed once a human approves the commit, and a pull
request is opened only once a human approves that too.

Without --once it ticks every --interval until interrupted, then stops its
runs at their next checkpoint for the next supervisor to resume. With --once
it does one pass and waits for the runs it started to halt.`,
	Args: cobra.NoArgs,
	RunE: runFactoryRun,
}

var factoryStatusCmd = &cobra.Command{
	Use: "status", Short: "List every order the factory has claimed, and where it is",
	Args: cobra.NoArgs, RunE: runFactoryStatus,
}

var factoryInboxCmd = &cobra.Command{
	Use: "inbox", Short: "List everything waiting on a human",
	Args: cobra.NoArgs, RunE: runFactoryInbox,
}

var factoryApproveCmd = &cobra.Command{
	Use:   "approve <order> <gate>",
	Short: "Approve an inbox item: a run's gate, commit, or open-pr",
	Long: `Records an approval bound to what the inbox shows now. For a run's gate the
supervisor then runs 'loom run --resume --approve <gate>', so the run's state is
exactly what approving it by hand would produce. If what the approval is bound
to changes before the supervisor acts, the approval is dropped and asked again.`,
	Args: cobra.ExactArgs(2), RunE: runFactoryApprove,
}

var factoryStopCmd = &cobra.Command{
	Use: "stop <order>", Short: "Interrupt a running order now; it checkpoints and waits for 'resume'",
	Args: cobra.ExactArgs(1), RunE: runFactoryStop,
}

var factoryResumeCmd = &cobra.Command{
	Use: "resume <order>", Short: "Continue a stopped order",
	Args: cobra.ExactArgs(1), RunE: runFactoryResume,
}

var factoryRerunCmd = &cobra.Command{
	Use: "rerun <order>", Short: "Re-run an order from its committed version, which changed since its claim",
	Args: cobra.ExactArgs(1), RunE: runFactoryRerun,
}

func init() {
	rootCmd.AddCommand(factoryCmd)
	factoryCmd.AddCommand(factoryRunCmd, factoryStatusCmd, factoryInboxCmd, factoryApproveCmd,
		factoryStopCmd, factoryResumeCmd, factoryRerunCmd)
	flags := factoryRunCmd.Flags()
	flags.BoolVar(&factoryArgs.once, "once", false, "do one pass, wait for the runs it started to halt, and exit (for cron)")
	flags.IntVar(&factoryArgs.maxConcurrent, "max-concurrent", factory.DefaultMaxConcurrent, "runs in flight at once")
	flags.DurationVar(&factoryArgs.interval, "interval", factory.DefaultInterval, "time between passes")
	flags.StringVar(&factoryArgs.branch, "branch", "main", "branch whose committed intake/ is read")
	flags.StringVar(&factoryArgs.provider, "provider", factory.DefaultProvider, "stage provider for every run: claude or mock")
	flags.DurationVar(&factoryArgs.retryBase, "retry-base", factory.DefaultRetryBase, "delay before the first retry; doubles each time")
	flags.DurationVar(&factoryArgs.retryMax, "retry-max", factory.DefaultRetryMax, "longest delay between retries")
	flags.StringArrayVar(&factoryArgs.runArgs, "run-arg", nil, "extra argument for every loom run (repeatable)")
	_ = flags.MarkHidden("run-arg")
}

// factoryRoot is the repository the factory runs in. The factory needs git,
// so outside a repository it is an error rather than the current directory.
func factoryRoot() (factory.Layout, error) {
	root, err := worktree.RepositoryRoot(".")
	if err != nil {
		return factory.Layout{}, fmt.Errorf("loom factory runs inside a git repository: %w", err)
	}
	return factory.Layout{Root: root}, nil
}

func runFactoryRun(cmd *cobra.Command, _ []string) error {
	layout, err := factoryRoot()
	if err != nil {
		return err
	}
	if err := layout.Prepare(); err != nil {
		return err
	}
	lock, err := factory.AcquireLock(layout.LockPath())
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	supervisor, err := newFactorySupervisor(cmd, layout)
	if err != nil {
		return err
	}
	if factoryArgs.once {
		return supervisor.Once()
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return supervisor.Run(ctx, factoryArgs.interval)
}

func newFactorySupervisor(cmd *cobra.Command, layout factory.Layout) (*factory.Supervisor, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find the loom binary: %w", err)
	}
	config := factory.Config{
		Layout: layout, IntakeBranch: factoryArgs.branch, MaxConcurrent: factoryArgs.maxConcurrent,
		Provider: factoryArgs.provider, RunArgs: factoryArgs.runArgs, Plans: runnablePlans(layout.Root),
		RetryBase: factoryArgs.retryBase, RetryMax: factoryArgs.retryMax,
	}
	return factory.New(config, factory.Dependencies{
		Repository: worktree.New(layout.Root), Launcher: factory.ExecLauncher{Executable: executable},
		Trees: openTree, Pulls: factory.GHPullRequests{}, Output: cmd.OutOrStdout(),
	}), nil
}

func openTree(dir string) factory.Tree { return worktree.New(dir) }

// openLedger returns the ledger and the orders folded from it.
func openLedger() (factory.Layout, *factory.Ledger, map[string]*factory.OrderState, error) {
	layout, err := factoryRoot()
	if err != nil {
		return layout, nil, nil, err
	}
	ledger := factory.OpenLedger(layout.LedgerPath())
	entries, err := ledger.Entries()
	if err != nil {
		return layout, nil, nil, err
	}
	return layout, ledger, factory.Replay(entries), nil
}

func runFactoryStatus(cmd *cobra.Command, _ []string) error {
	_, _, orders, err := openLedger()
	if err != nil {
		return err
	}
	if len(orders) == 0 {
		cmd.Println("No orders claimed yet.")
		return nil
	}
	table := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ORDER\tPHASE\tATTEMPT\tSINCE\tDETAIL")
	for _, order := range factory.Sorted(orders) {
		fmt.Fprintf(table, "%s\t%s\t%d\t%s\t%s\n", order.ID, order.Phase, order.Attempt,
			order.Since.Local().Format(time.DateTime), statusDetail(order))
	}
	return table.Flush()
}

func statusDetail(order *factory.OrderState) string {
	switch {
	case order.Phase == factory.PhaseWaiting:
		return "gate " + order.Gate
	case order.URL != "":
		return order.URL
	}
	return order.Reason
}

func readCommittedIntake(layout factory.Layout) (factory.Intake, error) {
	return factory.ReadIntake(worktree.New(layout.Root), factoryArgs.branch, runnablePlans(layout.Root))
}

func factoryInbox() (*factory.Ledger, []factory.InboxItem, error) {
	layout, ledger, orders, err := openLedger()
	if err != nil {
		return nil, nil, err
	}
	committed, err := readCommittedIntake(layout)
	if err != nil {
		return nil, nil, err
	}
	items, err := factory.Inbox(orders, committed, openTree)
	return ledger, items, err
}

func runFactoryInbox(cmd *cobra.Command, _ []string) error {
	_, items, err := factoryInbox()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		cmd.Println("Nothing is waiting on you.")
		return nil
	}
	for _, item := range items {
		printInboxItem(cmd, item)
	}
	return nil
}

func printInboxItem(cmd *cobra.Command, item factory.InboxItem) {
	cmd.Printf("%s  %s  (since %s)\n  %s\n", item.Order, item.Kind, item.Since.Local().Format(time.DateTime), item.Summary)
	for _, detail := range item.Details {
		cmd.Printf("    %s\n", detail)
	}
	if item.IsApprovable() {
		cmd.Printf("  binds to %s\n  approve: loom factory approve %s %s\n", shortDigest(item.Digest), item.Order, item.Gate)
	}
	cmd.Println()
}

func shortDigest(digest string) string {
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

func runFactoryApprove(cmd *cobra.Command, args []string) error {
	ledger, items, err := factoryInbox()
	if err != nil {
		return err
	}
	entry, err := factory.Approve(ledger, items, args[0], args[1], approverName())
	if err != nil {
		return err
	}
	cmd.Printf("Approved %s for %s (bound to %s). The supervisor acts on it at its next pass.\n",
		entry.Gate, entry.Order, shortDigest(entry.Digest))
	return nil
}

// approverName is who is approving, as the operating system knows them.
func approverName() string {
	if current, err := user.Current(); err == nil && current.Username != "" {
		return current.Username
	}
	return "unknown"
}

func runFactoryStop(cmd *cobra.Command, args []string) error {
	_, ledger, orders, err := openLedger()
	if err != nil {
		return err
	}
	if err := factory.RequestStop(ledger, orders, args[0]); err != nil {
		return err
	}
	cmd.Printf("Stopping %s: it checkpoints and waits for 'loom factory resume %s'.\n", args[0], args[0])
	return nil
}

func runFactoryResume(cmd *cobra.Command, args []string) error {
	_, ledger, orders, err := openLedger()
	if err != nil {
		return err
	}
	if err := factory.RequestResume(ledger, orders, args[0]); err != nil {
		return err
	}
	cmd.Printf("Resuming %s at the supervisor's next pass.\n", args[0])
	return nil
}

func runFactoryRerun(cmd *cobra.Command, args []string) error {
	layout, ledger, orders, err := openLedger()
	if err != nil {
		return err
	}
	committed, err := readCommittedIntake(layout)
	if err != nil {
		return err
	}
	if err := factory.RequestRerun(ledger, orders, committed, args[0]); err != nil {
		return err
	}
	cmd.Printf("Re-running %s from its committed version at the supervisor's next pass.\n", args[0])
	return nil
}
