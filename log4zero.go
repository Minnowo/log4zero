// Package log4zero provides named zerolog loggers that are configured at
// runtime from a JSON file, plus a way to dump every registered logger as a
// config so users can discover what exists.
package log4zero

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Values for LoggerConfig.Format. An empty Format means FormatConsole.
const (
	// FormatConsole writes human-readable text, colored according to Color.
	FormatConsole = "console"

	// FormatJSON writes one JSON object per line and ignores Color.
	FormatJSON = "json"
)

// LogCreator builds a logger for one config entry. cfg is the entry as written
// in the config (Level not normalized), level is cfg.Level parsed, and writer
// is the opened cfg.File, or stdout if File is empty. cfg.Format is
// already validated. Use cfg.Color.Enabled(writer) to resolve ColorAuto. It
// must not return nil.
type LogCreator func(name string, level zerolog.Level, writer io.Writer, cfg LoggerConfig) *zerolog.Logger

// LoggerConfig defines per-logger config.
//
// Format is "console" (the default when empty), which is human-readable and
// colored according to Color, or "json", which writes one JSON object per
// line and ignores Color.
type LoggerConfig struct {
	Level  string    `json:"level"`
	File   string    `json:"file,omitempty"`
	Color  ColorMode `json:"color,omitempty"`
	Format string    `json:"format,omitempty"`
}

// Config defines the overall config structure.
type Config struct {
	Loggers map[string]LoggerConfig `json:"loggers"`
}

// registryEntry is one named logger and the config it was last built from.
// The logger pointer is handed to users and must never be replaced, only
// overwritten in place, so that existing holders see config changes.
type registryEntry struct {
	logger *zerolog.Logger
	config LoggerConfig
}

// registryMu guards registry. It is never held while calling a user-supplied
// LogCreator, so a creator may safely call Get.
var registryMu sync.Mutex
var registry = map[string]*registryEntry{}

// once and onceErr hold InitOnce's state. onceErr is written inside once.Do,
// which orders it before every later read.
var once sync.Once
var onceErr error

// InitOnce initializes zerolog loggers from a JSON config file.
// This function uses a sync.Once to call Init, so it only ever works once.
// Every call returns the first call's error (nil on success); configPath is
// ignored after the first call. Safe for concurrent use.
func InitOnce(configPath string) error {

	once.Do(func() {
		onceErr = Init(configPath)
	})

	return onceErr
}

// Init populates and updates the logger registry with loggers from the given config file.
// Unknown fields in the file are an error, so typos do not pass silently.
func Init(configPath string) error {

	file, err := os.Open(configPath)

	if err != nil {
		return fmt.Errorf("could not open config file: %w", err)
	}
	defer file.Close()

	var cfg Config

	decoder := json.NewDecoder(file)

	// A misspelled field would otherwise be silently ignored.
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&cfg); err != nil {
		return fmt.Errorf("could not decode config: %w", err)
	}

	return InitWith(cfg, GetNew)
}

// InitWith populates and updates the logger registry with loggers from the given config using the given creation function.
func InitWith(cfg Config, createFunc LogCreator) error {

	for name, loggerCfg := range cfg.Loggers {

		level, err := zerolog.ParseLevel(loggerCfg.Level)

		if err != nil {
			return fmt.Errorf("invalid level for %s: %w", name, err)
		}

		if loggerCfg.Format != "" && loggerCfg.Format != FormatConsole && loggerCfg.Format != FormatJSON {
			return fmt.Errorf("invalid format for %s: %q, want %q or %q", name, loggerCfg.Format, FormatConsole, FormatJSON)
		}

		if !loggerCfg.Color.valid() {
			return fmt.Errorf("invalid color for %s: %d", name, int(loggerCfg.Color))
		}

		var writer io.Writer = os.Stdout

		if loggerCfg.File != "" {

			f, err := os.OpenFile(loggerCfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

			if err != nil {
				return fmt.Errorf("failed to open log file for %s: %w", name, err)
			}

			writer = f
		}

		newLogger := createFunc(name, level, writer, loggerCfg)

		if newLogger == nil {
			return fmt.Errorf("got nil logger from creation function")
		}

		storedCfg := loggerCfg
		// Init parses levels with LevelFieldMarshalFunc, so store the name in
		// that form for the dump to reload.
		storedCfg.Level = zerolog.LevelFieldMarshalFunc(level)

		registryMu.Lock()

		if existing, ok := registry[name]; ok {
			*existing.logger = *newLogger
			existing.config = storedCfg
		} else {
			registry[name] = &registryEntry{logger: newLogger, config: storedCfg}
		}

		registryMu.Unlock()
	}

	return nil
}

// Get returns a logger for a name, or a default info-level logger if not found.
func Get(name string) *zerolog.Logger {
	return GetL(name, zerolog.InfoLevel)
}

// GetL returns a logger for a name, or registers and returns a new stdout
// logger at the given level, using ColorAuto, if the name is not registered
// yet.
// Safe for concurrent use.
func GetL(name string, level zerolog.Level) *zerolog.Logger {

	registryMu.Lock()
	defer registryMu.Unlock()

	if existing, ok := registry[name]; ok {
		return existing.logger
	}

	cfg := LoggerConfig{Level: zerolog.LevelFieldMarshalFunc(level), Color: ColorAuto}

	logger := GetNew(name, level, os.Stdout, cfg)

	registry[name] = &registryEntry{logger: logger, config: cfg}

	return logger
}

// CurrentConfig returns a snapshot of every registered logger and the config
// it currently uses. Loggers created by Get/GetL without a config entry report
// their default level, stdout and ColorAuto. The returned Config is a copy and is
// safe to modify. Safe for concurrent use.
//
// A logger appears only once Get/GetL/InitWith has run for its name, so
// loggers fetched lazily inside functions are missing until first use.
// Package-level `var logger = log4zero.Get(...)` loggers are always present.
//
// Only the LoggerConfig fields are recorded. A logger built by InitWith with a
// custom LogCreator is reported by those fields alone, and reloading the dump
// through Init rebuilds it with GetNew.
func CurrentConfig() Config {

	registryMu.Lock()
	defer registryMu.Unlock()

	cfg := Config{Loggers: make(map[string]LoggerConfig, len(registry))}

	for name, entry := range registry {
		cfg.Loggers[name] = entry.config
	}

	return cfg
}

// WriteConfig writes CurrentConfig to w as indented JSON in the same format
// Init reads, so the output can be saved and edited as a config file. Intended
// for a CLI flag such as --dump-log-config. Returns any write error.
func WriteConfig(w io.Writer) error {

	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")

	if err := enc.Encode(CurrentConfig()); err != nil {
		return fmt.Errorf("could not encode config: %w", err)
	}

	return nil
}

// GetNew is the default LogCreator. It returns a logger at the given level
// that writes to writer with caller info and, if name is not empty, a
// logger=<name> field. cfg.Format "json" writes raw JSON lines; any other
// value writes console text (InitWith rejects unknown formats beforehand).
func GetNew(name string, level zerolog.Level, writer io.Writer, cfg LoggerConfig) *zerolog.Logger {

	if cfg.Format != FormatJSON {
		writer = zerolog.ConsoleWriter{Out: writer, NoColor: !cfg.Color.Enabled(writer)}
	}

	ctx := log.Output(writer).Level(level).With().Caller()

	if name != "" {
		ctx = ctx.Str("logger", name)
	}

	logger := ctx.Logger()

	return &logger
}
