// Tests for the color setting: config syntax, terminal detection and the
// escape codes that reach the output.
package log4zero

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"strings"
	"testing"
)

// ansiEscape starts every ANSI color sequence.
const ansiEscape = "\x1b["

// The config file accepts true, false and "auto", and dumps them back in the
// same form; anything else is an error from Init.
func TestColorModeJSON(t *testing.T) {

	valid := map[string]ColorMode{
		`true`:   ColorOn,
		`false`:  ColorOff,
		`"auto"`: ColorAuto,
	}

	for text, want := range valid {

		var got ColorMode

		if err := json.Unmarshal([]byte(text), &got); err != nil {
			t.Errorf("unmarshal %s: %v", text, err)
			continue
		}

		if got != want {
			t.Errorf("unmarshal %s: got %d, want %d", text, got, want)
		}

		out, err := json.Marshal(got)
		if err != nil {
			t.Errorf("marshal %d: %v", got, err)
			continue
		}

		if string(out) != text {
			t.Errorf("marshal %d: got %s, want %s", got, out, text)
		}
	}

	for _, text := range []string{`"yes"`, `"AUTO"`, `1`, `"true"`} {

		var got ColorMode

		if err := json.Unmarshal([]byte(text), &got); err == nil {
			t.Errorf("unmarshal %s: accepted, got %d", text, got)
		}
	}

	if _, err := json.Marshal(ColorMode(42)); err == nil {
		t.Errorf("marshal of undefined ColorMode succeeded")
	}
}

// Init loads "auto" from a file and rejects an invalid color value.
func TestInitColorAutoFromFile(t *testing.T) {

	useEmptyRegistry(t)

	dir := t.TempDir()

	good := path.Join(dir, "good.json")
	bad := path.Join(dir, "bad.json")

	writeFile(t, good, `{"loggers": {"color_auto": {"level": "info", "color": "auto"}}}`)
	writeFile(t, bad, `{"loggers": {"color_bad": {"level": "info", "color": "maybe"}}}`)

	if err := Init(good); err != nil {
		t.Fatalf("Init rejected color \"auto\": %v", err)
	}

	if got := CurrentConfig().Loggers["color_auto"].Color; got != ColorAuto {
		t.Errorf("color_auto has color %d, want ColorAuto", got)
	}

	if err := Init(bad); err == nil {
		t.Errorf("Init accepted color \"maybe\"")
	}
}

// InitWith rejects a ColorMode outside the defined constants.
func TestInitWithRejectsUndefinedColor(t *testing.T) {

	useEmptyRegistry(t)

	err := InitWith(Config{
		Loggers: map[string]LoggerConfig{"color_undefined": {Level: "info", Color: ColorMode(42)}},
	}, discardCreator)

	if err == nil || !strings.Contains(err.Error(), "invalid color") {
		t.Errorf("want invalid color error, got %v", err)
	}
}

// colorSupported is true only for a character device, and NO_COLOR or
// TERM=dumb turn it off even then.
func TestColorSupported(t *testing.T) {

	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm")

	regular, err := os.Create(path.Join(t.TempDir(), "regular.log"))
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer regular.Close()

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	defer pipeR.Close()
	defer pipeW.Close()

	if colorSupported(regular) {
		t.Errorf("regular file reported as color terminal")
	}

	if colorSupported(pipeW) {
		t.Errorf("pipe reported as color terminal")
	}

	if colorSupported(&bytes.Buffer{}) {
		t.Errorf("bytes.Buffer reported as color terminal")
	}

	if ColorAuto.Enabled(regular) {
		t.Errorf("ColorAuto colored a regular file")
	}

	if !ColorOn.Enabled(regular) {
		t.Errorf("ColorOn did not color a regular file")
	}

	// The null device is a character device, the same check a terminal
	// passes, so it stands in for a terminal here.
	device, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer device.Close()

	info, err := device.Stat()
	if err != nil {
		t.Fatalf("stat %s: %v", os.DevNull, err)
	}

	if info.Mode()&os.ModeCharDevice == 0 {
		t.Skipf("%s is not a character device on this platform", os.DevNull)
	}

	if !colorSupported(device) {
		t.Errorf("character device not reported as color terminal")
	}

	if !ColorAuto.Enabled(device) {
		t.Errorf("ColorAuto did not color a character device")
	}

	if ColorOff.Enabled(device) {
		t.Errorf("ColorOff colored a character device")
	}

	t.Setenv("NO_COLOR", "1")

	if colorSupported(device) {
		t.Errorf("NO_COLOR=1 did not disable color")
	}

	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")

	if colorSupported(device) {
		t.Errorf("TERM=dumb did not disable color")
	}
}

// Escape codes reach a log file only with ColorOn; ColorAuto sees a regular
// file and leaves color off, as does ColorOff.
func TestColorModeOutput(t *testing.T) {

	useEmptyRegistry(t)

	dir := t.TempDir()

	files := map[ColorMode]string{
		ColorOn:   path.Join(dir, "on.log"),
		ColorOff:  path.Join(dir, "off.log"),
		ColorAuto: path.Join(dir, "auto.log"),
	}

	cfg := Config{Loggers: map[string]LoggerConfig{}}

	for mode, file := range files {
		cfg.Loggers[file] = LoggerConfig{Level: "info", File: file, Color: mode}
	}

	if err := InitWith(cfg, GetNew); err != nil {
		t.Fatalf("InitWith failed: %v", err)
	}

	for mode, file := range files {

		Get(file).Info().Msg("hello color")

		lines := readLines(t, file)

		if len(lines) != 1 {
			t.Fatalf("want 1 line in %s, got %d: %q", file, len(lines), lines)
		}

		hasColor := strings.Contains(lines[0], ansiEscape)

		if hasColor != (mode == ColorOn) {
			t.Errorf("color mode %d: escape codes present = %v in %q", mode, hasColor, lines[0])
		}
	}
}

// writeFile writes content to file, failing the test on error.
func writeFile(t *testing.T, file string, content string) {

	t.Helper()

	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", file, err)
	}
}
