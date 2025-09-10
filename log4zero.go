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

// LoggerRegistry holds named loggers.
var LoggerRegistry = map[string]*zerolog.Logger{}
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

// Init populates and updates the LoggerRegistry with loggers from the given config file.
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

		newLogger := getDefault(name, level, writer, loggerCfg.Color)

		if existingLogger, ok := LoggerRegistry[name]; ok {
			*existingLogger = *newLogger
		} else {
			LoggerRegistry[name] = newLogger
		}
	}

	return nil
}

// Get returns a logger for a name, or a default info-level logger if not found.
func Get(name string) *zerolog.Logger {
	return GetL(name, zerolog.InfoLevel)
}

// Get returns a logger for a name with the given default level if not found.
func GetL(name string, level zerolog.Level) *zerolog.Logger {

	if logger, ok := LoggerRegistry[name]; ok {
		return logger
	}

	logger := getDefault(name, level, os.Stdout, true)

	LoggerRegistry[name] = logger

	return logger
}

func getDefault(name string, level zerolog.Level, writer io.Writer, color bool) *zerolog.Logger {

	writer = zerolog.ConsoleWriter{Out: writer, NoColor: !color}

	logger := log.Output(writer).Level(level).With().Caller().Str("logger", name).Logger()
	logger.Debug().Msg("logger created")

	return &logger
}
