package factory

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// LaunchRequest is one `loom run` invocation for an order.
type LaunchRequest struct {
	Dir     string
	Args    []string
	LogPath string
}

// Process is a running `loom run`.
type Process interface {
	PID() int
	// Done is closed when the process has exited.
	Done() <-chan struct{}
	// ExitCode is valid once Done is closed; -1 means killed by a signal.
	ExitCode() int
	// Interrupt asks the run to checkpoint and exit.
	Interrupt() error
}

// Launcher starts processes. The supervisor owns no other way to run code.
type Launcher interface {
	Launch(request LaunchRequest) (Process, error)
}

// ExecLauncher runs the loom binary.
type ExecLauncher struct {
	// Executable is the loom binary — normally the running one.
	Executable string
}

// Launch starts the process in its own process group, so a Ctrl-C meant for
// the supervisor does not also reach every run: the supervisor forwards one
// interrupt itself, and a second would kill a run mid-checkpoint.
func (l ExecLauncher) Launch(request LaunchRequest) (Process, error) {
	if err := os.MkdirAll(filepath.Dir(request.LogPath), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	log, err := os.OpenFile(request.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open run log: %w", err)
	}
	command := exec.Command(l.Executable, request.Args...)
	command.Dir, command.Stdout, command.Stderr = request.Dir, log, log
	command.SysProcAttr = ownProcessGroup()
	if err := command.Start(); err != nil {
		log.Close()
		return nil, fmt.Errorf("start %s: %w", l.Executable, err)
	}
	process := &execProcess{command: command, done: make(chan struct{})}
	go process.wait(log)
	return process, nil
}

type execProcess struct {
	command  *exec.Cmd
	done     chan struct{}
	once     sync.Once
	exitCode int
}

func (p *execProcess) wait(log *os.File) {
	_ = p.command.Wait()
	p.exitCode = p.command.ProcessState.ExitCode()
	log.Close()
	p.once.Do(func() { close(p.done) })
}

func (p *execProcess) PID() int              { return p.command.Process.Pid }
func (p *execProcess) Done() <-chan struct{} { return p.done }
func (p *execProcess) ExitCode() int         { return p.exitCode }
func (p *execProcess) Interrupt() error      { return interruptProcess(p.command.Process.Pid) }
