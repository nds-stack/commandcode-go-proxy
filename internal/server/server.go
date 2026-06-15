package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/nds-stack/commandcode-go-proxy/internal/proxy"
)

const defaultPort = "9173"
const defaultHost = "127.0.0.1"

var requestCounter uint64

var colors = []string{
	"\033[32m", // green
	"\033[36m", // cyan
	"\033[33m", // yellow
	"\033[35m", // magenta
	"\033[34m", // blue
	"\033[31m", // red
	"\033[92m", // light green
	"\033[96m", // light cyan
	"\033[93m", // light yellow
	"\033[95m", // light magenta
}

const resetColor = "\033[0m"

func nextColor() string {
	n := atomic.AddUint64(&requestCounter, 1)
	return colors[(int(n)-1)%len(colors)]
}

// Server represents the HTTP server
type Server struct {
	Port    string
	Host    string
	Proxy   *proxy.Proxy
	Handler http.Handler
}

// NewServer creates a new server instance
func NewServer(proxy *proxy.Proxy) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", logger(proxy.HandleChatCompletions))
	mux.HandleFunc("/chat/completions", logger(proxy.HandleChatCompletions))
	mux.HandleFunc("/v1/responses", logger(proxy.HandleResponses))
	mux.HandleFunc("/v1/models", logger(proxy.HandleModels))
	mux.HandleFunc("/health", logger(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	return &Server{
		Port:    defaultPort,
		Host:    defaultHost,
		Proxy:   proxy,
		Handler: mux,
	}
}

// SetPort sets the port for the server
func (s *Server) SetPort(port string) {
	if port != "" {
		s.Port = port
	}
}

// SetHost sets the host for the server
func (s *Server) SetHost(host string) {
	if host != "" {
		s.Host = host
	}
}

// GetPort returns the server port
func (s *Server) GetPort() string {
	return s.Port
}

// GetHost returns the server host
func (s *Server) GetHost() string {
	return s.Host
}

// logger is a middleware for logging requests with per-request colors
func logger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		color := nextColor()
		start := time.Now()
		fmt.Printf("%s[%s] %s %s%s\n", color, r.Method, r.URL.Path, r.RemoteAddr, resetColor)
		next(w, r)
		fmt.Printf("%s[%s] %s done in %v%s\n", color, r.Method, r.URL.Path, time.Since(start), resetColor)
	}
}

// Start starts the HTTP server with graceful shutdown
func (s *Server) Start() {
	addr := s.Host + ":" + s.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Handler,
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  0,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server stopped")
}
