# Kjor

A live reload tool focused on stability and reliability over speed.

*Note*: For now, this project only supports linux and the fanotify
protocol.

## Install

`go install github.com/subfusc/kjor@latest`

## Usage

Executing `kjor` in a go project root directory will default to
running `go build -o a.out ./` and `./a.out`. A default config file
called `kjor.toml` is created in the root directory.

kjor monitors all files in all directories that are not hidden except
the ones matching the regular expressions in the ignore part of the
config. It is a good idea to ignore the resulting executable in order
to avoid an inifite loop if the compile times becomes larger than 1
second.

## Config

The default config:

```TOML
  [Program]
    Name = "./a.out"
    Args = []

  [Build]
    Name = "go"
    Args = ["build", "-o", "a.out", "./"]

  [Filewatcher]
    Ignore = ["^\\.#", "^#", "~$", "_test\\.go$", "a\\.out$"]

  [SSE]
    Enable = true
    Port = 8888
```

## Dependencies

- Fanotify v3
