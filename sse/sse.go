package sse

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

type Event struct {
	Type   string
	Source uint
	When   time.Time
	Data   map[string]any
}

func (e Event) ToMessage() string {
	e.Data["When"] = e.When

	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(e.Data)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", e.Type, buf.String())
}

type Server struct {
	logger         *slog.Logger
	srv            *http.Server
	MsgChan        chan Event
	RestartTimeout int
}

func sseHeaders(h http.Header) {
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("X-Accel-Buffering", "no")
}

func (s *Server) SSETrapper() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("Opening socket")
		sseHeaders(w.Header())
		sse := w.(http.Flusher)
		defer func() { s.logger.Info("Closing socket") }()

		lastSent := time.Now()

		delayed := struct {
			Ctx  context.Context
			Ctl  context.CancelFunc
			Done bool
		}{Done: true}

		delayedSend := func(c context.Context, message Event) {
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
			case message := <-s.MsgChan:
				switch message.Source {
				case WATCHER:
					if message.Type == "build_action" &&  message.Data["restarted"] != nil && delayed.Done {
						delayed.Ctx, delayed.Ctl = context.WithTimeout(context.Background(), time.Duration(s.RestartTimeout)*time.Millisecond)
						delayed.Done = false
						go delayedSend(delayed.Ctx, message)
					} else {
						fmt.Fprintf(w, message.ToMessage())
						sse.Flush()
					}
				case DEV_SERVER:
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
				return
			}
		}
	}
}

func NewServer(c *config.Config, logger *slog.Logger) *Server {
	mux := &http.ServeMux{}
	sseServer := &Server{
		logger: logger,
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", c.SSE.Port),
			Handler: mux,
		},
		RestartTimeout: c.SSE.RestartTimeout,
	}

	mux.HandleFunc("GET /listen", sseServer.SSETrapper())
	mux.HandleFunc("POST /started", func(w http.ResponseWriter, r *http.Request) {
		if cap(sseServer.MsgChan) > len(sseServer.MsgChan) {
			sseServer.MsgChan <- Event{Type: "build_action", Source: DEV_SERVER, When: time.Now(), Data: map[string]any{"restarted": true}}
		}
	})
	mux.HandleFunc("GET /listener.js",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript")
			w.Write([]byte(fmt.Sprintf(`
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
        const eventSrc = new EventSource("http://localhost:%d/listen")

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
      `, c.SSE.Port)))
		}))
	return sseServer
}

func (s *Server) Start() {
	s.logger.Info("Starting server", "Addr", s.srv.Addr)
	s.MsgChan = make(chan Event, 1)
	s.srv.ListenAndServe()
}

func (s *Server) Close() {
	close(s.MsgChan)
	s.srv.Close()
}
