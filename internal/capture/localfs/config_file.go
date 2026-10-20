package localfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/orieken/loom/internal/capture"
	"go.yaml.in/yaml/v4"
)

// ErrInvalidConfig marks a settings file that cannot be used. The hook then
// writes nothing and counts a failure: a default would drop the extra
// redaction globs of the user (architecture-notes.md decision 4).
var ErrInvalidConfig = errors.New("localfs: invalid capture settings")

// maxConfigBytes caps the settings file. A larger file is invalid.
const maxConfigBytes = 64 << 10

var _ capture.ConfigSource = ConfigFile{}

// ConfigFile loads .claude/capture.yaml. Defaults are compiled in, and a
// missing file is not a failure.
type ConfigFile struct {
	path string
}

// NewConfigFile builds a loader for the given path.
func NewConfigFile(path string) ConfigFile {
	return ConfigFile{path: path}
}

// settingsDocument lists every key of the file. A key that is not here makes
// the file invalid. A pointer tells "not set" from "set to zero".
type settingsDocument struct {
	Window        *string         `yaml:"window"`
	RetentionDays *int            `yaml:"retentionDays"`
	CapMegabytes  *int64          `yaml:"capMegabytes"`
	Weights       map[string]int  `yaml:"weights"`
	Signals       map[string]bool `yaml:"signals"`
	Redact        []string        `yaml:"redact"`
}

// Load reads the file over the defaults.
func (c ConfigFile) Load() (capture.Config, error) {
	content, err := c.readFile()
	if errors.Is(err, fs.ErrNotExist) {
		return capture.DefaultConfig(), nil
	}
	if err != nil {
		return capture.Config{}, invalid(err)
	}
	return parseSettings(content)
}

func (c ConfigFile) readFile() ([]byte, error) {
	info, err := os.Stat(c.path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxConfigBytes {
		return nil, errors.New("not a regular file or too large")
	}
	return os.ReadFile(c.path)
}

func parseSettings(content []byte) (capture.Config, error) {
	var document settingsDocument
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil && !errors.Is(err, io.EOF) {
		return capture.Config{}, invalid(err)
	}
	return document.applyTo(capture.DefaultConfig())
}

func invalid(cause error) error {
	return fmt.Errorf("%w: %v", ErrInvalidConfig, cause)
}

// applyTo checks every set value and lays it over the defaults.
func (d settingsDocument) applyTo(cfg capture.Config) (capture.Config, error) {
	steps := []func(capture.Config) (capture.Config, error){
		d.applyWindow, d.applyRetention, d.applyWeights, d.applySignals, d.applyRedact,
	}
	var err error
	for _, step := range steps {
		if cfg, err = step(cfg); err != nil {
			return capture.Config{}, invalid(err)
		}
	}
	return cfg, nil
}

func (d settingsDocument) applyWindow(cfg capture.Config) (capture.Config, error) {
	if d.Window == nil {
		return cfg, nil
	}
	window, err := time.ParseDuration(*d.Window)
	if err != nil || window <= 0 {
		return cfg, errors.New("window must be a positive duration")
	}
	cfg.Window = window
	return cfg, nil
}

func (d settingsDocument) applyRetention(cfg capture.Config) (capture.Config, error) {
	if d.RetentionDays != nil {
		cfg.Retention.MaxAgeDays = *d.RetentionDays
	}
	if d.CapMegabytes != nil {
		cfg.Retention.MaxBytes = *d.CapMegabytes * capture.BytesPerMegabyte
	}
	if cfg.Retention.MaxAgeDays < 1 || cfg.Retention.MaxBytes < capture.BytesPerMegabyte {
		return cfg, errors.New("retentionDays and capMegabytes must be at least 1")
	}
	return cfg, nil
}

func (d settingsDocument) applyWeights(cfg capture.Config) (capture.Config, error) {
	targets := map[capture.CorrectionKind]*int{
		capture.KindHunkEdit:         &cfg.Weights.HunkEdit,
		capture.KindRevert:           &cfg.Weights.Revert,
		capture.KindDeniedCall:       &cfg.Weights.DeniedCall,
		capture.KindCorrectivePrompt: &cfg.Weights.CorrectivePrompt,
	}
	for name, minutes := range d.Weights {
		target, isKnown := targets[capture.CorrectionKind(name)]
		if !isKnown || minutes < 0 {
			return cfg, errors.New("weights: unknown kind or negative value")
		}
		*target = minutes
	}
	return cfg, nil
}

func (d settingsDocument) applySignals(cfg capture.Config) (capture.Config, error) {
	for name, isOn := range d.Signals {
		signal := capture.Signal(name)
		if _, isKnown := cfg.Signals[signal]; !isKnown {
			return cfg, errors.New("signals: unknown signal")
		}
		cfg.Signals[signal] = isOn
	}
	return cfg, nil
}

func (d settingsDocument) applyRedact(cfg capture.Config) (capture.Config, error) {
	for _, pattern := range d.Redact {
		if err := capture.ValidateGlob(pattern); err != nil {
			return cfg, errors.New("redact: bad glob")
		}
	}
	cfg.RedactionGlobs = d.Redact
	return cfg, nil
}
