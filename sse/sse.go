package sse

// Thanks to this blog:
// https://dev.to/mirzaakhena/server-sent-events-sse-server-implementation-with-go-4ck2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type Event struct {
	Type string
	Data any
}

func (e Event) ToMessage() string {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(e.Data)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", e.Type, buf.String())
}

type Server struct {
	srv *http.Server
	MsgChan chan Event
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
		s.MsgChan = make(chan Event)
		defer func() {
			close(s.MsgChan)
			s.MsgChan = nil
			slog.Info("SSE socket closed")
		}()

		sse := w.(http.Flusher)
		for {
			select {
			case message := <- s.MsgChan:
				slog.Info("Got event", "msg", message)
				fmt.Fprint(w, message.ToMessage())
				sse.Flush()
			case <- r.Context().Done():
				return
			}
		}
	}
}

func NewServer() *Server {
	mux := &http.ServeMux{}
	sseServer := &Server{
		srv: &http.Server{
			Addr: ":8888",
			Handler: mux,
		},
	}

	mux.HandleFunc("GET /listen", sseServer.SSETrapper())
	mux.HandleFunc("GET /listener.js",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript")
			w.Write([]byte(`
        const eventSrc = new EventSource("http://localhost:8888/listen")
        eventSrc.addEventListener("server_message", (event) => {
          console.log(event.data)
          window.location.reload()
        })
      `))
		}))
	return sseServer
}

func (s *Server) Start() {
	slog.Info("Starting SSE server", "Addr", s.srv.Addr)
	s.srv.ListenAndServe()
}

func (s *Server) Close() {
	s.srv.Close()
}
