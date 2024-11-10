package webserver

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type WebServer struct {
	Router        chi.Router
	Handlers      map[string]http.HandlerFunc
	Middlewares   []func(http.Handler) http.Handler
	WebServerPort string
	httpServer    *http.Server
}

func NewWebServer(serverPort string) *WebServer {
	return &WebServer{
		Router:        chi.NewRouter(),
		Handlers:      make(map[string]http.HandlerFunc),
		Middlewares:   make([]func(http.Handler) http.Handler, 0),
		WebServerPort: serverPort,
	}
}

func (s *WebServer) AddHandler(path string, handler http.HandlerFunc) {
	s.Handlers[path] = handler
}

func (s *WebServer) AddMiddleware(handler func(next http.Handler) http.Handler) {
	s.Middlewares = append(s.Middlewares, handler)
}

// loop through the handlers and add them to the router
// register middeleware logger
// start the server
func (s *WebServer) Start() {
	s.Router.Use(middleware.Logger)
	for _, middleware := range s.Middlewares {
		s.Router.Use(middleware)
	}
	for path, handler := range s.Handlers {
		s.Router.Handle(path, handler)
	}
	server := &http.Server{Addr: s.WebServerPort, Handler: s.Router}
	s.httpServer = server
	go func() {
		log.Printf("Server is running at http://localhost%s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && http.ErrServerClosed != err {
			log.Fatalf("Could not listen on %s: %v\n", server.Addr, err)
		}
	}()
}

func (s *WebServer) Stop(ctx context.Context) {
	log.Println("Shutting down server...")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Could not gracefully shutdown the server: %v\n", err)
	}
	log.Println("Server stopped")
}
