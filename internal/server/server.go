package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/think-root/threads-connector/internal/config"
	"github.com/think-root/threads-connector/internal/threads"
)

type Server struct {
	Config *config.Config
	Client *threads.Client
}

func New(cfg *config.Config, client *threads.Client) *Server {
	return &Server{
		Config: cfg,
		Client: client,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Health check - no auth, no logging
	mux.HandleFunc("/health", s.handleHealth)

	// Wrap with logging and auth middleware
	handler := s.loggingMiddleware(s.authMiddleware(s.handlePost))
	mux.HandleFunc("/threads/post", handler)

	log.Printf("Starting server on port %s", s.Config.Port)
	return http.ListenAndServe(fmt.Sprintf(":%s", s.Config.Port), mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" || apiKey != s.Config.APIKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

type postRequest struct {
	Text     string `json:"text"`
	ImageURL string `json:"image_url"`
	URL      string `json:"url"`
}

type postResponse struct {
	PostID string `json:"post_id"`
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req postRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Basic validation: must have text OR image
	if req.Text == "" && req.ImageURL == "" {
		http.Error(w, "Content (text or image_url) is required", http.StatusBadRequest)
		return
	}

	textSnippet := req.Text
	if len(textSnippet) > 50 {
		textSnippet = textSnippet[:50] + "..."
	}
	log.Printf("Processing post request. Text: %q (len=%d), Image: %v, URL: %s", 
		textSnippet, len(req.Text), req.ImageURL != "", req.URL)

	postID, err := s.Client.CreatePost(req.Text, req.ImageURL, req.URL)
	if err != nil {
		log.Printf("Error creating post: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create post: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully created post: %s", postID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(postResponse{PostID: postID})
}

func (s *Server) loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request for %s", r.Method, r.URL.Path)
		next(w, r)
	}
}
