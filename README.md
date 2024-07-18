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

### Browser reloader

If SSE is enabled, the browser can also be notfied of changes. The
changes will be broadcasted on an SSE socket under `/listen`. The
default port is 8888, so the full url for a dev environment is
`http://localhost:8888/listen`.

A small JS to just reload the browser tab every time the server is
restarted is found under `/listener.js` and can be included in your
dev template body simply by adding the following script tag in `<body>`:

```HTML
	<script src="http://localhost:8888/listener.js"></script>
```

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
