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

*Note*: this feature is experimental. I will have to see if I find this
usefull or not.

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

The RestartTimeout variable is there in case you need to delay the
refresh until the server has been started. As an alternative, if the
load time of your server is very long, you can make the server post to
`http://localhost:8888/started` to trigger a reload exactly when your
server is ready.

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
  RestartTimeout = 1000
```

## A note on containers

This was primarily written to be run inside containers, but seccomp
have some interesting defaults which seemingly will stop you from
using fanotify in a good way. Atleast for this usecase.

## Dependencies

- Fanotify v3 or inotify
