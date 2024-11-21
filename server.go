package main

import (
	"fmt"
	"log"
	"net/http"
)

// Server encapsulates the HTTP server configuration.
type Server struct {
	SlackHandler *SlackHandler
	Port         string
}

// NewServer creates a new instance of Server.
func NewServer(slackHandler *SlackHandler, port string) *Server {
	return &Server{
		SlackHandler: slackHandler,
		Port:         port,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() {
	http.HandleFunc("/events-endpoint", s.SlackHandler.HandleEventEndpoint)
	log.Printf("[INFO] Server listening on port %s", s.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", s.Port), nil); err != nil {
		log.Fatalf("[ERROR] Server failed: %v", err)
	}
}
