package log4zero

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestInitConsoleLog(t *testing.T) {

	name := "test_1"

	config := Config{
		Loggers: map[string]LoggerConfig{
			name: {
				Level: "info",
				File:  "",
				Color: true,
			},
		},
	}

	InitWith(config, GetNew)

	logger := Get(name)
	logger.Info().Msg("hello world")
}

func TestInitFileLog(t *testing.T) {

	name := "test_2"
	temp := t.TempDir()
	file := path.Join(temp, "output_file_2.log")

	config := Config{
		Loggers: map[string]LoggerConfig{
			name: {
				Level: "info",
				File:  file,
				Color: true,
			},
		},
	}

	InitWith(config, GetNew)

	logger := Get(name)
	logger.Info().Msg("hello world")

	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		t.Error("log file does not exist")
		t.Fail()
	}
}

func TestInitWithCustomLog(t *testing.T) {

	name := "test_3"
	temp := t.TempDir()
	file := path.Join(temp, "output_file_3.log")

	config := Config{
		Loggers: map[string]LoggerConfig{
			name: {
				Level: "info",
				File:  file,
				Color: true,
			},
		},
	}

	InitWith(config, func(name string, level zerolog.Level, writer io.Writer, color bool) *zerolog.Logger {

		logger := log.Output(zerolog.MultiLevelWriter(writer, os.Stdout)).Level(level)

		return &logger
	})

	logger := Get(name)
	logger.Error().Msg("hello world")

	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		t.Errorf("log file does not exist: %v", err)
		t.Fail()
	}
}

// This is to test that you can Get the logger once, and never need to get it again.
// The logger you get is a pointer, and it's pointed value changes when the Init function updates the logger.
func TestInitManyTimes(t *testing.T) {

	name := "test_3"
	temp := t.TempDir()
	file := path.Join(temp, "output_file_3.log")

	// Get the logger BEFORE any Init, default level should be INFO
	logger := Get(name)

	// should NOT be logged
	logger.Debug().Msg("before init")

	// Init with debug level
	InitWith(Config{
		Loggers: map[string]LoggerConfig{
			name: {
				Level: "debug",
				File:  file,
				Color: false,
			},
		},
	}, GetNew)

	// Should be logged
	logger.Debug().Msg("during debug")

	// Init with error level
	InitWith(Config{
		Loggers: map[string]LoggerConfig{
			name: {
				Level: "error",
				File:  file,
				Color: false,
			},
		},
	}, GetNew)

	// Should NOT be logged
	logger.Debug().Msg("after error")

	// should be logged
	logger.Error().Msg("actual error")

	content, err := os.ReadFile(file)

	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logOutput := string(content)

	if strings.Contains(logOutput, "before init") {
		t.Errorf("log before InitWith should NOT appear")
	}
	if !strings.Contains(logOutput, "during debug") {
		t.Errorf("log during debug should appear")
	}
	if strings.Contains(logOutput, "after error") {
		t.Errorf("debug log after setting level to error should NOT appear")
	}
	if !strings.Contains(logOutput, "actual error") {
		t.Errorf("error log after setting level to error should appear")
	}

}
