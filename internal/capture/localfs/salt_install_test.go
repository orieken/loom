package localfs

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/iotest"

	"github.com/orieken/loom/internal/capture"
)

func saltPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), ".claude", "memory", "capture.salt")
}

func TestInstallSaltCreatesAPrivate32ByteFile(t *testing.T) {
	path := saltPath(t)
	random := bytes.NewReader(bytes.Repeat([]byte{7}, 64))
	outcome, err := InstallSalt(path, random)
	if err != nil || outcome != SaltCreated {
		t.Fatalf("outcome = %v, err = %v", outcome, err)
	}
	content, _ := os.ReadFile(path)
	if !bytes.Equal(content, bytes.Repeat([]byte{7}, capture.SaltBytes)) {
		t.Fatalf("salt = %x, want 32 bytes taken from the random source", content)
	}
}

func TestInstallSaltFileAndDirectoryArePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes only")
	}
	path := saltPath(t)
	if _, err := InstallSalt(path, nil); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	dir, _ := os.Stat(filepath.Dir(path))
	if info.Mode().Perm() != 0o600 || dir.Mode().Perm()&0o077 != 0 {
		t.Fatalf("file mode %v, directory mode %v", info.Mode().Perm(), dir.Mode().Perm())
	}
}

func TestInstallSaltNeverRotatesAnExistingSalt(t *testing.T) {
	path := saltPath(t)
	if _, err := InstallSalt(path, bytes.NewReader(bytes.Repeat([]byte{1}, 32))); err != nil {
		t.Fatal(err)
	}
	outcome, err := InstallSalt(path, bytes.NewReader(bytes.Repeat([]byte{2}, 32)))
	if err != nil || outcome != SaltKept {
		t.Fatalf("outcome = %v, err = %v", outcome, err)
	}
	if content, _ := os.ReadFile(path); !bytes.Equal(content, bytes.Repeat([]byte{1}, 32)) {
		t.Fatal("the second install changed the salt")
	}
}

func TestInstallSaltKeepsEvenACorruptExistingFile(t *testing.T) {
	path := saltPath(t)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte("short"), 0o600)
	outcome, err := InstallSalt(path, nil)
	if err != nil || outcome != SaltKept {
		t.Fatalf("outcome = %v, err = %v", outcome, err)
	}
	if content, _ := os.ReadFile(path); string(content) != "short" {
		t.Fatal("an existing file was overwritten")
	}
}

func TestInstallSaltUsesCryptoRandWhenNoSourceIsGiven(t *testing.T) {
	first, second := saltPath(t), saltPath(t)
	_, _ = InstallSalt(first, nil)
	_, _ = InstallSalt(second, nil)
	one, _ := os.ReadFile(first)
	two, _ := os.ReadFile(second)
	if len(one) != capture.SaltBytes || bytes.Equal(one, two) || bytes.Equal(one, make([]byte, 32)) {
		t.Fatal("the default source gave a fixed or short salt")
	}
}

func TestInstallSaltLeavesNoPartialFileWhenRandomFails(t *testing.T) {
	path := saltPath(t)
	_, err := InstallSalt(path, iotest.ErrReader(errors.New("no entropy")))
	if err == nil {
		t.Fatal("a failing random source should fail the install")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("a partial salt stayed behind")
	}
	if _, err := InstallSalt(path, bytes.NewReader(bytes.Repeat([]byte{3}, 32))); err != nil {
		t.Fatalf("retry after a failure: %v", err)
	}
}

func TestInstallSaltReportsAnUncreatableDirectory(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(blocker, nil, 0o600)
	if _, err := InstallSalt(filepath.Join(blocker, "memory", "capture.salt"), nil); err == nil {
		t.Fatal("expected an error")
	}
}
