package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type ApplicationConfig struct {
	Port        string        `json:"port"`
	Host        string        `json:"host"`
	Timeout     time.Duration `json:"timeout"`
	EnableDebug bool          `json:"enable_debug"`
}

type Server struct {
	config *ApplicationConfig
	router *http.ServeMux
}

func NewServer(config *ApplicationConfig) *Server {
	return &Server{
		config: config,
		router: http.NewServeMux(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	s.setupRoutes()

	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.Timeout,
		WriteTimeout: s.config.Timeout,
	}

	log.Printf("Server starting on %s", addr)
	return server.ListenAndServe()
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/health", s.healthHandler)
	s.router.HandleFunc("/api/users", s.usersHandler)
	s.router.HandleFunc("/api/data", s.dataHandler)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) usersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getUsersHandler(w, r)
	case http.MethodPost:
		s.createUserHandler(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) getUsersHandler(w http.ResponseWriter, r *http.Request) {
	users := []string{"alice", "bob", "charlie"}
	fmt.Fprintf(w, "Users: %v", users)
}

func (s *Server) createUserHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("User created"))
}

func (s *Server) dataHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"status":    "active",
	}
	fmt.Fprintf(w, "Data: %+v", data)
}

func main() {
	config := &ApplicationConfig{
		Port:        getEnv("PORT", "8080"),
		Host:        getEnv("HOST", "localhost"),
		Timeout:     30 * time.Second,
		EnableDebug: getEnv("DEBUG", "false") == "true",
	}

	server := NewServer(config)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}