package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"simsexam/internal/app"
	"simsexam/internal/config"
)

func main() {
	cfg := config.LoadServerConfig()

	serverApp, err := app.NewServerApp(context.Background(), cfg)
	if err != nil {
		log.Fatalf("Failed to initialize server app: %v", err)
	}
	defer serverApp.Close()

	fmt.Printf("Server starting on http://%s\n", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, serverApp.Router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
