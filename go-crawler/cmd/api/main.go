package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go-crawler/internal/crawler"
)

type CrawlRequest struct {
	URL    string        `json:"url"`
	Depth  int           `json:"depth"`
	Workers int          `json:"workers"`
	Delay  time.Duration `json:"delay"`
}

type CrawlResponse struct {
	Type    string      `json:"type"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type APIServer struct {
	crawler     *crawler.Crawler
	clients     map[*websocket.Conn]bool
	clientsLock sync.Mutex
	router      *mux.Router
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow connections from any origin for development
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewAPIServer() *APIServer {
	srv := &APIServer{
		clients: make(map[*websocket.Conn]bool),
		router:  mux.NewRouter(),
	}

	// Serve static files
	staticDir := "./web/static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		// Try to find the static directory relative to the executable
		exe, err := os.Executable()
		if err != nil {
			log.Fatal("Could not determine executable path:", err)
		}
		exeDir := filepath.Dir(exe)
		staticDir = filepath.Join(exeDir, "../../web/static")
	}

	// Register routes
	srv.router.HandleFunc("/ws", srv.handleWebSocket)
	srv.router.HandleFunc("/crawl", srv.handleCrawl).Methods("POST")
	srv.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	srv.router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	return srv
}

func (s *APIServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Println("New WebSocket connection request from:", r.RemoteAddr)
	
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Register client
	s.clientsLock.Lock()
	s.clients[conn] = true
	s.clientsLock.Unlock()

	log.Printf("Client connected. Total clients: %d", len(s.clients))

	// Send initial welcome message
	welcome := CrawlResponse{
		Type:    "status",
		Message: "Connected to crawler server",
	}
	if err := conn.WriteJSON(welcome); err != nil {
		log.Printf("Error sending welcome message: %v", err)
	}

	// Handle messages from client
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		log.Printf("Received message: %+v", msg)

		// Handle different message types
		switch msg["type"].(string) {
		case "start":
			// Handle start crawl request
			s.handleStartCrawl(conn, msg)
		case "stop":
			// Handle stop crawl request
			// You can implement this based on your requirements
			log.Println("Received stop request")
		}
	}

	// Unregister client
	s.clientsLock.Lock()
	delete(s.clients, conn)
	s.clientsLock.Unlock()
	log.Printf("Client disconnected. Remaining clients: %d", len(s.clients))
}

func (s *APIServer) handleStartCrawl(conn *websocket.Conn, msg map[string]interface{}) {
	// Parse the request
	startURL, _ := msg["url"].(string)
	depth, _ := msg["depth"].(float64)
	workers, _ := msg["workers"].(float64)
	delay, _ := msg["delay"].(float64)

	log.Printf("Starting crawl: url=%s, depth=%d, workers=%d, delay=%dms", 
		startURL, int(depth), int(workers), int(delay))

	// Validate URL
	_, err := url.ParseRequestURI(startURL)
	if err != nil {
		errMsg := fmt.Sprintf("Invalid URL: %v", err)
		errResp := CrawlResponse{
			Type:    "error",
			Message: errMsg,
		}
		if err := conn.WriteJSON(errResp); err != nil {
			log.Printf("Error sending error response: %v", err)
		}
		return
	}

	// Send acknowledgment
	ack := CrawlResponse{
		Type:    "start",
		Message: "Crawl started",
		Data: map[string]interface{}{
			"url":     startURL,
			"depth":   depth,
			"workers": workers,
			"delay":   delay,
		},
	}
	if err := conn.WriteJSON(ack); err != nil {
		log.Printf("Error sending ack: %v", err)
		return
	}

	// Start the crawl in a goroutine
	go func() {
		// Create a new crawler instance
		c := crawler.NewCrawler(int(workers), int(depth), time.Duration(delay)*time.Millisecond)
		
		// Create a context that we can cancel
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		// Start crawling
		results := c.Start(ctx, startURL)
		
		// Process results
		for result := range results {
			// Create a response with the crawl result
			respData := map[string]interface{}{
				"url":    result.URL,
				"status": "Crawled successfully",
			}
			
			// Add links if available
			if len(result.Links) > 0 {
				respData["links"] = result.Links
			}
			
			// Add error if present
			if result.Error != nil {
				respData["status"] = "Error"
				respData["error"] = result.Error.Error()
			}
			
			resp := CrawlResponse{
				Type: "result",
				Data: respData,
			}
			
			// Send the result
			if err := conn.WriteJSON(resp); err != nil {
				log.Printf("Error sending result: %v", err)
				return
			}
			
			// Small delay to prevent overwhelming the client
			time.Sleep(50 * time.Millisecond)
		}

		// Send completion message
		complete := CrawlResponse{
			Type:    "complete",
			Message: "Crawl completed",
			Data: map[string]interface{}{
				"url":       startURL,
				"pagesCrawled": c.VisitedCount(),
			},
		}
		if err := conn.WriteJSON(complete); err != nil {
			log.Printf("Error sending completion: %v", err)
		}
	}()
}

func (s *APIServer) broadcast(message CrawlResponse) {
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()

	for client := range s.clients {
		if err := client.WriteJSON(message); err != nil {
			log.Printf("Error broadcasting message: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
}

func (s *APIServer) handleCrawl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Depth <= 0 {
		req.Depth = 2
	}
	if req.Workers <= 0 {
		req.Workers = 5
	}
	if req.Delay <= 0 {
		req.Delay = 100 * time.Millisecond
	}

	// Initialize crawler if not already done
	s.crawler = crawler.NewCrawler(req.Workers, req.Depth, req.Delay)

	// Start crawling in a goroutine
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		s.broadcast(CrawlResponse{
			Type:    "status",
			Message: fmt.Sprintf("Starting crawl of %s with depth %d", req.URL, req.Depth),
		})


		results := s.crawler.Start(ctx, req.URL)

		for result := range results {
			if result.Error != nil {
				s.broadcast(CrawlResponse{
					Type:    "error",
					Message: fmt.Sprintf("Error crawling %s: %v", result.URL, result.Error),
				})
				continue
			}

			s.broadcast(CrawlResponse{
				Type: "result",
				Data: map[string]interface{}{
					"url":   result.URL,
					"links": result.Links,
				},
			})
		}

		s.broadcast(CrawlResponse{
			Type:    "status",
			Message: "Crawl completed",
		})
	}()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "Crawl started",
	})
}

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "Port to run the server on")
	workers := flag.Int("workers", 5, "Number of worker goroutines")
	depth := flag.Int("depth", 2, "Maximum crawl depth")
	delay := flag.Duration("delay", 100*time.Millisecond, "Delay between requests")
	flag.Parse()

	// Create a new crawler instance
	c := crawler.NewCrawler(*workers, *depth, *delay)

	// Create and start the API server
	server := NewAPIServer()
	server.crawler = c

	// Set up HTTP server
	server.router = mux.NewRouter()
	server.router.HandleFunc("/ws", server.handleWebSocket)
	server.router.HandleFunc("/crawl", server.handleCrawl).Methods("POST")

	// Serve static files from the web/static directory
	staticDir := "./web/static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		// Try to find the static directory relative to the executable
		executable, err := os.Executable()
		if err != nil {
			log.Fatal("Could not determine executable path:", err)
		}
		exeDir := filepath.Dir(executable)
		staticDir = filepath.Join(exeDir, "../../web/static")
	}

	server.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	server.router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	// Start the server
	addr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:    addr,
		Handler: server.router,
	}

	// Start the server in a goroutine
	log.Printf("Starting server on %s\n", addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Gracefully shut down the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v\n", err)
	}

	log.Println("Server exiting")
}
