package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"aigateway/internal/config"
	"aigateway/internal/middleware"
	"aigateway/internal/proxy"
	"aigateway/internal/store"
	"aigateway/internal/web"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to MySQL
	db, err := store.New(cfg.MySQL.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()
	log.Println("Connected to MySQL")

	// Auto-create schema
	if err := db.InitSchema(); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}
	log.Println("Database schema initialized")

	// Build HTTP mux
	mux := http.NewServeMux()

	// Admin UI and API
	webHandler, err := web.NewHandler(db, cfg)
	if err != nil {
		log.Fatalf("Failed to create web handler: %v", err)
	}
	mux.Handle("/admin", webHandler)
	mux.Handle("/admin/", webHandler)
	mux.Handle("/api/logs", webHandler)

	// Proxy routes
	for _, route := range cfg.Routes {
		proxyHandler, err := proxy.NewHandler(route.Prefix, route.BaseUrl)
		if err != nil {
			log.Fatalf("Failed to create proxy for %s: %v", route.Prefix, err)
		}
		// Wrap with logging middleware
		loggedHandler := middleware.Logging(db, route.Prefix, proxyHandler)
		// Register pattern: /prefix/... matches everything under the prefix
		pattern := route.Prefix + "/"
		mux.Handle(pattern, loggedHandler)
		// Also handle the exact prefix path (no trailing slash)
		mux.Handle(route.Prefix, loggedHandler)
		log.Printf("Route registered: %s -> %s", route.Prefix, route.BaseUrl)
	}

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		srv.Close()
	}()

	log.Printf("AI Gateway listening on %s", addr)
	log.Printf("Admin UI: http://localhost%s/admin", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped")
}
