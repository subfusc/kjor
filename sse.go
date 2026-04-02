package main

// Thanks to this blog:
// https://dev.to/mirzaakhena/server-sent-events-sse-server-implementation-with-go-4ck2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/subfusc/kjor/config"
)

const (
	WATCHER = iota
	DEV_SERVER
)

const listenerPayload = `
function hide(e) {
  div = document.getElementById("kjor-messages")
  div.style.display = "none"
}

function addMessageNode() {
  messageOutput = document.getElementById("kjor-messages")
  if (messageOutput == null) {
    body = document.getElementsByTagName("body")[0]
    div = document.createElement("div")
    div.id = "kjor-messages"
    div.style.backgroundColor = "orange"
    div.style.position = "fixed"
    div.style.left = "calc(50vw - 200px)"
    div.style.top = "0px"
    div.style.padding = "5px"
    div.style.borderRadius = "5px"
    div.style.display = "none"
    div.style.width = "400px"
    div.style.alignItems = "center"
    div.style.justifyContent = "space-between"
    div.style.flexDirection = "row"
    div.style.cursor = "pointer"
    div.onclick = hide
    body.appendChild(div)
  }
}

addMessageNode()
const eventSrc = new EventSource("http://" + document.domain + ":%d/listen")

eventSrc.addEventListener("build_action", (event) => {
  data = JSON.parse(event.data)
  if (data["restarted"]) {
    eventSrc.close()
    window.location.reload()
  }
})

eventSrc.addEventListener("build_message", (event) => {
  data = JSON.parse(event.data)
  if (data["message"] != null) {
    msg = document.getElementById("kjor-messages")
    msg.innerHTML = "<p>" + data["message"] + "</p><div class=\"kjor-close\">Ⓧ</div>"
    msg.style.display = "flex"
  }
})
`

type SSEEvent struct {
	Type   string
	Source uint
	When   time.Time
	Data   map[string]any
}

func (e SSEEvent) ToMessage() string {
	e.Data["When"] = e.When

	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(e.Data)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", e.Type, buf.String())
}

type SSEServer struct {
	logger         *slog.Logger
	srv            *http.Server
	Messages       chan SSEEvent
	RestartTimeout int
	mainCtx        context.Context
}

func setSSEHeaders(h http.Header) {
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("X-Accel-Buffering", "no")
}

func (s *SSEServer) Trapper() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("Opening SSE socket")

		setSSEHeaders(w.Header())
		sse := w.(http.Flusher)
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error("Socket closed badly", "err", err)
			}
			s.logger.Info("Closing socket")
		}()

		lastSent := time.Now()

		delayed := struct {
			Ctx  context.Context
			Ctl  context.CancelFunc
			Done bool
		}{Done: true}

		delayedSend := func(c context.Context, message SSEEvent) {
			<-c.Done()
			switch c.Err() {
			case context.Canceled:
			case context.DeadlineExceeded:
				if lastSent.Add(1 * time.Second).Before(message.When) {
					fmt.Fprint(w, message.ToMessage())
					sse.Flush()
					lastSent = time.Now()
				}
			}

			delayed.Done = true
		}

		for {
			select {
			case message := <-s.Messages:
				//RESTART
				switch message.Source {
				case WATCHER:
					if message.Type == "build_action" && message.Data["restarted"] != nil && delayed.Done {
						delayed.Ctx, delayed.Ctl = context.WithTimeout(context.Background(), time.Duration(s.RestartTimeout)*time.Millisecond)
						delayed.Done = false
						go delayedSend(delayed.Ctx, message)
					} else {
						fmt.Fprintf(w, message.ToMessage())
						sse.Flush()
					}
				case DEV_SERVER:
					// If dev_server sends message before build service, cancel the timeout and send
					// the reload message to SSE directly
					if !delayed.Done {
						delayed.Ctl()
					}

					if lastSent.Add(1 * time.Second).Before(message.When) {
						fmt.Fprint(w, message.ToMessage())
						sse.Flush()
						lastSent = time.Now()
					}
				}
			case <-r.Context().Done():
				return // Socket closed
			case <-s.mainCtx.Done():
				return // Main is done
			}
		}
	})
}

func (s *SSEServer) startMsg() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		event := SSEEvent{
			Type:   "build_action",
			Source: DEV_SERVER,
			When:   time.Now(),
			Data:   map[string]any{"restarted": true},
		}

		select {
		case s.Messages <- event:
		default:
		}
	})
}

func (s *SSEServer) listenerScript(port int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprintf(w, listenerPayload, port)
	})
}

func NewSSEServer(ctx context.Context, c *config.Config, logger *slog.Logger) *SSEServer {
	mux := &http.ServeMux{}
	sseServer := &SSEServer{
		logger: logger,
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", c.SSE.Port),
			Handler: mux,
		},
		RestartTimeout: c.SSE.RestartTimeout,
		Messages:       make(chan SSEEvent),
		mainCtx:        ctx,
	}

	mux.Handle("GET /listen", sseServer.Trapper())
	mux.Handle("POST /started", sseServer.startMsg())
	mux.Handle("GET /listener.js", sseServer.listenerScript(c.SSE.Port))

	return sseServer
}

func (s *SSEServer) Start() {
	s.logger.Info("Starting server", "Addr", s.srv.Addr)

	go func() {
		s.srv.ListenAndServe()
		defer func() {
			close(s.Messages)
			s.srv.Close()
		}()

		<-s.mainCtx.Done()
	}()
}
