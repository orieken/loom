package capture

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestSaltNeverAppearsInFormattedOutput(t *testing.T) {
	salt := newTestSalt(t)
	secrets := []string{testSaltText, "0123456789abcdef", hex.EncodeToString([]byte(testSaltText))}
	type holder struct{ S Salt }
	outputs := map[string]string{
		"%v":        fmt.Sprintf("%v", salt),
		"%+v":       fmt.Sprintf("%+v", salt),
		"%#v":       fmt.Sprintf("%#v", salt),
		"%s":        fmt.Sprintf("%s", salt),
		"%x":        fmt.Sprintf("%x", salt),
		"%q":        fmt.Sprintf("%q", salt),
		"%d":        fmt.Sprintf("%d", salt),
		"Sprint":    fmt.Sprint(salt),
		"pointer":   fmt.Sprintf("%v", &salt),
		"struct":    fmt.Sprintf("%+v", holder{S: salt}),
		"hasher":    fmt.Sprintf("%+v", NewHasher(salt)),
		"hasher #v": fmt.Sprintf("%#v", NewHasher(salt)),
	}
	for name, output := range outputs {
		for _, secret := range secrets {
			if strings.Contains(output, secret) {
				t.Errorf("%s output %q holds the salt", name, output)
			}
		}
		if !strings.Contains(output, "[redacted]") {
			t.Errorf("%s output %q does not say [redacted]", name, output)
		}
	}
}

func TestSaltStringMethodsDoNotShowTheKey(t *testing.T) {
	salt := newTestSalt(t)

	if salt.String() != "[redacted]" || salt.GoString() != "[redacted]" {
		t.Errorf("String=%q GoString=%q", salt.String(), salt.GoString())
	}
	hasher := NewHasher(salt)
	if hasher.String() != "Hasher[redacted]" || hasher.GoString() != "Hasher[redacted]" {
		t.Errorf("Hasher String=%q GoString=%q", hasher.String(), hasher.GoString())
	}
}

func TestSaltNeverAppearsInJSON(t *testing.T) {
	encoded, err := json.Marshal(struct{ S Salt }{S: newTestSalt(t)})

	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "0123456789") || !strings.Contains(string(encoded), "[redacted]") {
		t.Errorf("json = %s", encoded)
	}
}

func TestNewSaltRejectsWrongLength(t *testing.T) {
	for _, length := range []int{0, 1, SaltBytes - 1, SaltBytes + 1} {
		if _, err := NewSalt(make([]byte, length)); err != ErrBadSalt {
			t.Errorf("length %d: error = %v, want ErrBadSalt", length, err)
		}
	}
}

func TestNewSaltCopiesItsInput(t *testing.T) {
	raw := []byte(testSaltText)
	salt, _ := NewSalt(raw)
	before := NewHasher(salt).Hash(HashPath, "x")

	raw[0] = 'Z'

	if after := NewHasher(salt).Hash(HashPath, "x"); after != before {
		t.Error("changing the input slice changed the salt")
	}
}
