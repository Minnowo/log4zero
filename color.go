// Color settings for console output: the ColorMode config type and the
// terminal detection behind ColorAuto.
package log4zero

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ColorMode says whether console-format output is colored. In a config file
// it is written as true (ColorOn), false (ColorOff) or "auto" (ColorAuto).
// The zero value is ColorOff, so an omitted "color" field means no color.
type ColorMode int

const (
	// ColorOff never colors output.
	ColorOff ColorMode = iota

	// ColorOn always colors output, even when written to a file or pipe.
	ColorOn

	// ColorAuto colors output only when it goes to a terminal and color is
	// not disabled by the NO_COLOR or TERM=dumb conventions. See
	// colorSupported.
	ColorAuto
)

// colorAutoJSON is how ColorAuto is spelled in a config file.
const colorAutoJSON = "auto"

// UnmarshalJSON accepts true, false or "auto". JSON null leaves the value
// unchanged. Anything else is an error.
func (c *ColorMode) UnmarshalJSON(data []byte) error {

	text := string(data)

	switch text {
	case "null":
		return nil
	case "true":
		*c = ColorOn
		return nil
	case "false":
		*c = ColorOff
		return nil
	}

	var s string

	if err := json.Unmarshal(data, &s); err == nil && s == colorAutoJSON {
		*c = ColorAuto
		return nil
	}

	return fmt.Errorf("invalid color %s, want true, false or %q", text, colorAutoJSON)
}

// MarshalJSON writes ColorOn as true, ColorOff as false and ColorAuto as
// "auto", the same forms UnmarshalJSON reads. Any other value is an error.
func (c ColorMode) MarshalJSON() ([]byte, error) {

	switch c {
	case ColorOff:
		return []byte("false"), nil
	case ColorOn:
		return []byte("true"), nil
	case ColorAuto:
		return json.Marshal(colorAutoJSON)
	}

	return nil, fmt.Errorf("invalid ColorMode %d", int(c))
}

// valid reports whether c is one of the defined ColorMode values.
func (c ColorMode) valid() bool {
	return c == ColorOff || c == ColorOn || c == ColorAuto
}

// Enabled reports whether output written to w should be colored: always for
// ColorOn, never for ColorOff, and for ColorAuto only if w is a terminal and
// color is not disabled by the environment (see ColorAuto). Undefined values
// report false. Custom LogCreators use this to resolve cfg.Color.
func (c ColorMode) Enabled(w io.Writer) bool {

	switch c {
	case ColorOn:
		return true
	case ColorAuto:
		return colorSupported(w)
	}

	return false
}

// colorSupported reports whether w is a terminal that should get color.
// It follows https://no-color.org (a non-empty NO_COLOR disables color) and
// treats TERM=dumb as no color. Only an *os.File that is a character device
// counts as a terminal, so regular files, pipes and in-memory writers are
// never colored. It does not enable ANSI handling on legacy Windows consoles.
func colorSupported(w io.Writer) bool {

	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}

	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	info, err := f.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}
