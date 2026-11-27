// Tests for logger registration, config loading and config dumping.
package log4zero

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"strings"
	"sync"
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

// Get and InitWith are called from many goroutines at once. Run with -race;
// any unsynchronized access to the registry is reported as a failure.
func TestConcurrentGetAndInit(t *testing.T) {

	useEmptyRegistry(t)

	const workers = 8

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {

		name := fmt.Sprintf("concurrent_%d", i)

		wg.Add(3)

		go func() {
			defer wg.Done()
			Get(name)
		}()

		go func() {
			defer wg.Done()
			CurrentConfig()
		}()

		go func() {
			defer wg.Done()
			err := InitWith(Config{
				Loggers: map[string]LoggerConfig{name: {Level: "warn"}},
			}, discardCreator)
			if err != nil {
				t.Errorf("InitWith failed: %v", err)
			}
		}()
	}

	wg.Wait()
}

// WriteConfig must list every registered logger, both ones created by Get
// with defaults and ones set by InitWith, and its output must be loadable by
// Init.
func TestWriteConfigDumpsAllLoggers(t *testing.T) {

	useEmptyRegistry(t)

	file := path.Join(t.TempDir(), "dump.log")

	Get("dump_default")

	err := InitWith(Config{
		Loggers: map[string]LoggerConfig{
			"dump_configured": {Level: "DEBUG", File: file, Color: false},
		},
	}, discardCreator)
	if err != nil {
		t.Fatalf("InitWith failed: %v", err)
	}

	var buf bytes.Buffer

	if err := WriteConfig(&buf); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	var got Config

	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("WriteConfig output is not valid JSON: %v\n%s", err, buf.String())
	}

	want := Config{
		Loggers: map[string]LoggerConfig{
			"dump_default":    {Level: "info", Color: true},
			"dump_configured": {Level: "debug", File: file, Color: false},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("dumped config mismatch\ngot:  %+v\nwant: %+v", got, want)
	}

	configPath := path.Join(t.TempDir(), "config.json")

	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	if err := Init(configPath); err != nil {
		t.Fatalf("Init could not load WriteConfig output: %v", err)
	}

	if reloaded := CurrentConfig(); !reflect.DeepEqual(reloaded, want) {
		t.Errorf("config changed after reload\ngot:  %+v\nwant: %+v", reloaded, want)
	}
}

// A dumped config must reload even when the app customizes level names,
// because Init parses levels with zerolog.LevelFieldMarshalFunc.
func TestWriteConfigRoundTripsCustomLevelNames(t *testing.T) {

	useEmptyRegistry(t)

	savedMarshal := zerolog.LevelFieldMarshalFunc
	zerolog.LevelFieldMarshalFunc = func(l zerolog.Level) string {
		return strings.ToUpper(l.String()[:3])
	}
	t.Cleanup(func() { zerolog.LevelFieldMarshalFunc = savedMarshal })

	err := InitWith(Config{
		Loggers: map[string]LoggerConfig{"custom_levels": {Level: "WAR"}},
	}, discardCreator)
	if err != nil {
		t.Fatalf("InitWith failed: %v", err)
	}

	var buf bytes.Buffer

	if err := WriteConfig(&buf); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	var dumped Config

	if err := json.Unmarshal(buf.Bytes(), &dumped); err != nil {
		t.Fatalf("WriteConfig output is not valid JSON: %v", err)
	}

	if err := InitWith(dumped, discardCreator); err != nil {
		t.Errorf("dumped config does not reload: %v\n%s", err, buf.String())
	}
}

// discardCreator is a LogCreator that drops all output, to keep test logs quiet.
func discardCreator(name string, level zerolog.Level, writer io.Writer, color bool) *zerolog.Logger {
	logger := zerolog.New(io.Discard).Level(level)
	return &logger
}

// useEmptyRegistry gives the test an empty logger registry and restores the
// previous one when the test ends, so registry contents are deterministic.
func useEmptyRegistry(t *testing.T) {

	registryMu.Lock()
	saved := registry
	registry = map[string]*registryEntry{}
	registryMu.Unlock()

	t.Cleanup(func() {
		registryMu.Lock()
		registry = saved
		registryMu.Unlock()
	})
}
