// Package log4zero provides named zerolog loggers that are configured at
// runtime from a JSON file.
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

type LogCreator func(name string, level zerolog.Level, writer io.Writer, color bool) *zerolog.Logger

// LoggerConfig defines per-logger config.
type LoggerConfig struct {
	Level string `json:"level"`
	File  string `json:"file,omitempty"`
	Color bool   `json:"color,omitempty"`
}

// Config defines the overall config structure.
type Config struct {
	Loggers map[string]LoggerConfig `json:"loggers"`
}

// registry holds named loggers. The pointers are handed to users and must
// never be replaced, only overwritten in place, so that existing holders see
// config changes.
//
// registryMu guards registry. It is never held while calling a user-supplied
// LogCreator, so a creator may safely call Get.
var registryMu sync.Mutex
var registry = map[string]*zerolog.Logger{}

var once sync.Once

// InitOnce initializes zerolog loggers from a JSON config file.
// This function uses a sync.Once to call Init, so it only ever works once.
func InitOnce(configPath string) error {

	var err error

	once.Do(func() {
		err = Init(configPath)
	})

	return err
}

// Init populates and updates the logger registry with loggers from the given config file.
func Init(configPath string) error {

	file, err := os.Open(configPath)

	if err != nil {
		return fmt.Errorf("could not open config file: %w", err)
	}
	defer file.Close()

	var cfg Config

	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
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

		var writer io.Writer = os.Stdout

		if loggerCfg.File != "" {

			f, err := os.OpenFile(loggerCfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

			if err != nil {
				return fmt.Errorf("failed to open log file for %s: %w", name, err)
			}

			writer = f
		}

		newLogger := createFunc(name, level, writer, loggerCfg.Color)

		if newLogger == nil {
			return fmt.Errorf("got nil logger from creation function")
		}

		registryMu.Lock()

		if existingLogger, ok := registry[name]; ok {
			*existingLogger = *newLogger
		} else {
			registry[name] = newLogger
		}

		registryMu.Unlock()
	}

	return nil
}

// Get returns a logger for a name, or a default info-level logger if not found.
func Get(name string) *zerolog.Logger {
	return GetL(name, zerolog.InfoLevel)
}

// GetL returns a logger for a name, or registers and returns a new colored
// stdout logger at the given level if the name is not registered yet.
// Safe for concurrent use.
func GetL(name string, level zerolog.Level) *zerolog.Logger {

	registryMu.Lock()
	defer registryMu.Unlock()

	if logger, ok := registry[name]; ok {
		return logger
	}

	logger := GetNew(name, level, os.Stdout, true)

	registry[name] = logger

	return logger
}

// GetNew creates a new logger
func GetNew(name string, level zerolog.Level, writer io.Writer, color bool) *zerolog.Logger {

	writer = zerolog.ConsoleWriter{Out: writer, NoColor: !color}

	ctx := log.Output(writer).Level(level).With().Caller()

	if name != "" {
		ctx = ctx.Str("logger", name)
	}

	logger := ctx.Logger()

	logger.Debug().Msg("logger created")

	return &logger
}
