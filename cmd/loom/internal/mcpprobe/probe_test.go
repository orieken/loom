package mcpprobe

import (
	"context"
	"testing"
	"time"
)

func TestToolsListFailures(t *testing.T) {
	cases := []struct {
		name    string
		command string
		args    []string
	}{
		{name: "command does not exist", command: "loom-mcpprobe-no-such-command"},
		{name: "server exits without answering", command: "sh", args: []string{"-c", "exit 0"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := ToolsList(ctx, t.TempDir(), testCase.command, testCase.args...); err == nil {
				t.Fatal("ToolsList should fail")
			}
		})
	}
}

func TestToolsListParsesAdvertisedTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The fake server answers, then holds stdin open until the probe closes
	// it — exactly what a real MCP server does. It must be a FOREGROUND read:
	// POSIX assigns an asynchronous command's stdin to /dev/null before any
	// explicit redirection, so `cat >/dev/null &` reads /dev/null and never
	// the pipe. With the read in the background, nothing holds the read end
	// once sh exits, and the probe's writes race that exit — winning on a
	// fast machine and losing on a loaded CI runner with EPIPE.
	script := `echo '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"alpha"},{"name":"beta"}]}}'; cat >/dev/null`
	names, err := ToolsList(ctx, t.TempDir(), "sh", "-c", script)
	if err != nil {
		t.Fatalf("ToolsList: %v", err)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("names = %v, want [alpha beta]", names)
	}
}
