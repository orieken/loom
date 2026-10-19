package capture

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func referenceHash(kind HashKind, value string) string {
	mac := hmac.New(sha256.New, []byte(testSaltText))
	mac.Write([]byte(string(kind) + "\x00" + value))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

func TestHasherMatchesTheAcceptedScheme(t *testing.T) {
	hasher := NewHasher(newTestSalt(t))
	for _, kind := range []HashKind{HashPath, HashLines, HashSession} {
		if got, want := hasher.Hash(kind, "src/a.go"), referenceHash(kind, "src/a.go"); got != want {
			t.Errorf("kind %q: hash = %s, want %s", kind, got, want)
		}
	}
}

func TestHasherOutputIs32HexDigits(t *testing.T) {
	got := NewHasher(newTestSalt(t)).Hash(HashPath, "x")

	if len(got) != HashBytes*2 {
		t.Errorf("len = %d, want %d", len(got), HashBytes*2)
	}
}

func TestHasherSeparatesKindsAndSalts(t *testing.T) {
	hasher := NewHasher(newTestSalt(t))
	otherSalt, _ := NewSalt([]byte("ffffffffffffffffffffffffffffffff"))

	if hasher.Hash(HashPath, "a") == hasher.Hash(HashSession, "a") {
		t.Error("equal values of different kinds must hash differently")
	}
	if hasher.Hash(HashPath, "a") == NewHasher(otherSalt).Hash(HashPath, "a") {
		t.Error("equal values under different salts must hash differently")
	}
	if hasher.Hash(HashPath, "ab") == hasher.Hash(HashPath, "a") {
		t.Error("different values must hash differently")
	}
}

func TestHasherResetsBetweenCalls(t *testing.T) {
	hasher := NewHasher(newTestSalt(t))

	first := hasher.Hash(HashPath, "a")
	hasher.Hash(HashPath, "b")

	if again := hasher.Hash(HashPath, "a"); again != first {
		t.Errorf("hash after another call = %s, want %s", again, first)
	}
}

func TestHashLineRange(t *testing.T) {
	hasher := NewHasher(newTestSalt(t))
	tests := []struct {
		name       string
		content    string
		start, end int
		wantText   string
		isFound    bool
	}{
		{"middle lines", "a\nb\nc\nd\n", 2, 3, "b\nc", true},
		{"first line", "a\nb\n", 1, 1, "a", true},
		{"last line without final newline", "a\nb", 2, 2, "b", true},
		{"windows line endings hash like unix", "a\r\nb\r\nc\r\n", 2, 3, "b\nc", true},
		{"start below one", "a\nb\n", 0, 1, "", false},
		{"end before start", "a\nb\n", 2, 1, "", false},
		{"end past the last line", "a\nb\n", 1, 3, "", false},
		{"empty content", "", 1, 1, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, isFound := hasher.HashLineRange([]byte(tt.content), tt.start, tt.end)
			if isFound != tt.isFound {
				t.Fatalf("found = %v, want %v", isFound, tt.isFound)
			}
			if isFound && got != hasher.Hash(HashLines, tt.wantText) {
				t.Errorf("hash does not match the text %q", tt.wantText)
			}
		})
	}
}

func TestHashLineRangeSeesAChangeInOneLine(t *testing.T) {
	hasher := NewHasher(newTestSalt(t))

	before, _ := hasher.HashLineRange([]byte("a\nb\nc\n"), 1, 3)
	after, _ := hasher.HashLineRange([]byte("a\nB\nc\n"), 1, 3)

	if before == after {
		t.Error("a changed line must change the hash")
	}
}
