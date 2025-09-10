
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
            "color": false,
            "file" : "./main.log"
        },
        "<your-logger2-name>": {
            "level": "info",
            "color": true,
        }
    }
}
```

The `color` and `file` fields are optional.


