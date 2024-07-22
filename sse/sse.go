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
	Data   any
}

func (e Event) ToMessage() string {
	var data map[string]any

	switch ie := e.Data.(type) {
	case map[string]any:
		ie["When"] = e.When
		data = ie
	default:
		data = map[string]any{
			"When":    e.When,
			"Message": ie,
		}
	}

	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(data)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", e.Type, buf.String())
}

type Server struct {
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
		slog.Info("SSE opening socket")
		sseHeaders(w.Header())
		sse := w.(http.Flusher)
		defer func() { slog.Info("Closing SSE socket") }()

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
					if delayed.Done {
						delayed.Ctx, delayed.Ctl = context.WithTimeout(context.Background(), time.Duration(s.RestartTimeout)*time.Millisecond)
						delayed.Done = false
						go delayedSend(delayed.Ctx, message)
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

func NewServer(c *config.Config) *Server {
	mux := &http.ServeMux{}
	sseServer := &Server{
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", c.SSE.Port),
			Handler: mux,
		},
		RestartTimeout: c.SSE.RestartTimeout,
	}

	mux.HandleFunc("GET /listen", sseServer.SSETrapper())
	mux.HandleFunc("POST /started", func(w http.ResponseWriter, r *http.Request) {
		if cap(sseServer.MsgChan) > len(sseServer.MsgChan) {
			sseServer.MsgChan <- Event{Type: "server_message", Source: DEV_SERVER, When: time.Now(), Data: map[string]any{"restarted": true}}
		}
	})
	mux.HandleFunc("GET /listener.js",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript")
			w.Write([]byte(fmt.Sprintf(`
        const eventSrc = new EventSource("http://localhost:%d/listen")
        eventSrc.addEventListener("server_message", (event) => {
          console.log(event.data)
          eventSrc.close()
          window.location.reload()
        })
      `, c.SSE.Port)))
		}))
	return sseServer
}

func (s *Server) Start() {
	slog.Info("Starting SSE server", "Addr", s.srv.Addr)
	s.MsgChan = make(chan Event, 1)
	s.srv.ListenAndServe()
}

func (s *Server) Close() {
	close(s.MsgChan)
	s.srv.Close()
}
