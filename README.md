
# Log4zero

Log4zero is a very simple log library built on [ZeroLog](https://github.com/rs/zerolog).

It provides the bare-minimum to be somewhat similar to log4j, in the sense that you can set loggers at runtime from a file.

## Usage

```go
package main

import (
	"github.com/minnowo/log4zero"
)

// get a logger with the name
var logger = log4zero.Get("main")

func main() {

    // read the config file at this location
	log4zero.Init("./log-config.json")

    // use your logger
	logger.Info().Msg("hello world")
}

```

## Configuration

The configuration file looks something like:
```json
{
    "loggers": {
        "<your-logger1-name>": {
            "level": "debug",
            "file" : "./main.log",
            "format": "json"
        },
        "<your-logger2-name>": {
            "level": "info",
            "color": true
        }
    }
}
```

Always set `level`: an omitted level is not an error, but the logger then
drops nearly everything. The other fields are optional. Values are lowercase
and `format` is case-sensitive.

| Field    | Default   | Meaning                                                    |
|----------|-----------|------------------------------------------------------------|
| `level`  |           | `trace`, `debug`, `info`, `warn`, `error`, `fatal`, ...    |
| `file`   | stdout    | Append log output to this file                             |
| `format` | `console` | `console` for human-readable text, `json` for JSON lines   |
| `color`  | `false`   | Colorize `console` output; ignored for `json`              |

Unknown fields are an error, so a typo such as `"levle"` makes `Init` fail
instead of being silently ignored.


## Discovering loggers

Rather than documenting every logger name by hand, let users ask the program.
`WriteConfig` writes every registered logger, with its current settings, as a
config file that `Init` can read:

```go
dump := flag.Bool("dump-log-config", false, "print the logging config and exit")
flag.Parse()

if *dump {
	if err := log4zero.WriteConfig(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
```

```sh
./myapp --dump-log-config > log-config.json   # then edit levels as needed
```

`CurrentConfig()` returns the same data as a `Config` value if you want to
inspect it in code.

A logger only shows up once `Get`, `GetL` or `Init` has run for its name.
Loggers declared as package-level variables (`var logger = log4zero.Get("x")`)
are created at startup, so they are always included. A logger fetched inside a
function will not be listed until that function has run.

`Get`, `GetL`, `CurrentConfig` and `WriteConfig` are safe for concurrent use.
Calling `Init` again while other goroutines are logging is not: it updates each
logger in place, so do it at startup or while logging is quiet.
