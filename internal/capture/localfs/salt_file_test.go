package localfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/capture"
)

func cancelContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx, cancel
}

func TestSaltFileReadsAValidSalt(t *testing.T) {
	path := writeTemp(t, "capture.salt", strings.Repeat("s", capture.SaltBytes))

	salt, err := NewSaltFile(path).Salt()

	if err != nil {
		t.Fatal(err)
	}
	// The salt must be usable: hashing proves the 32 bytes were read.
	want := capture.NewHasher(mustSalt(t, strings.Repeat("s", capture.SaltBytes))).Hash(capture.HashPath, "x")
	if got := capture.NewHasher(salt).Hash(capture.HashPath, "x"); got != want {
		t.Fatalf("hash = %s, want %s", got, want)
	}
}

func mustSalt(t *testing.T, raw string) capture.Salt {
	t.Helper()
	salt, err := capture.NewSalt([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return salt
}

func TestSaltFileTypedErrors(t *testing.T) {
	thirtyTwo := strings.Repeat("s", capture.SaltBytes)
	tests := []struct {
		name    string
		prepare func(t *testing.T) string
		want    error
	}{
		{name: "missing file", prepare: func(t *testing.T) string { return filepath.Join(t.TempDir(), "none") }, want: ErrSaltMissing},
		{name: "31 bytes", prepare: func(t *testing.T) string { return writeTemp(t, "s", thirtyTwo[:31]) }, want: ErrSaltCorrupt},
		{name: "33 bytes", prepare: func(t *testing.T) string { return writeTemp(t, "s", thirtyTwo+"x") }, want: ErrSaltCorrupt},
		{name: "empty", prepare: func(t *testing.T) string { return writeTemp(t, "s", "") }, want: ErrSaltCorrupt},
		{name: "a directory", prepare: func(t *testing.T) string { return t.TempDir() }, want: ErrSaltCorrupt},
		{name: "group readable", prepare: func(t *testing.T) string { return chmodSalt(t, thirtyTwo, 0o640) }, want: ErrSaltMode},
		{name: "world readable", prepare: func(t *testing.T) string { return chmodSalt(t, thirtyTwo, 0o644) }, want: ErrSaltMode},
		{name: "world writable only", prepare: func(t *testing.T) string { return chmodSalt(t, thirtyTwo, 0o602) }, want: ErrSaltMode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.prepare(t)

			_, err := NewSaltFile(path).Salt()

			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if err != nil && strings.Contains(fmt.Sprint(err), thirtyTwo) {
				t.Fatal("the error must not carry salt bytes")
			}
		})
	}
}

func chmodSalt(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	path := writeTemp(t, "capture.salt", content)
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSaltFileAcceptsAReadOnlyOwnerFile(t *testing.T) {
	path := chmodSalt(t, strings.Repeat("s", capture.SaltBytes), 0o400)

	if _, err := NewSaltFile(path).Salt(); err != nil {
		t.Fatalf("err = %v, want a 0400 salt accepted", err)
	}
}

func TestSaltFileNeverCreatesTheSalt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.salt")

	_, _ = NewSaltFile(path).Salt()

	if _, err := os.Stat(path); err == nil {
		t.Fatal("a missing salt must stay missing: a new salt breaks every stored hash")
	}
}

func TestReadSaltRejectsAShortReader(t *testing.T) {
	if _, err := readSalt(strings.NewReader("short")); !errors.Is(err, ErrSaltCorrupt) {
		t.Fatalf("err = %v, want ErrSaltCorrupt", err)
	}
	if _, err := readSalt(strings.NewReader(strings.Repeat("s", 40))); !errors.Is(err, ErrSaltCorrupt) {
		t.Fatalf("err = %v, want ErrSaltCorrupt for a reader longer than the salt", err)
	}
}
