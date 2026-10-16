package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/taolicus/asteroids/internal/rooms"
	"github.com/taolicus/asteroids/internal/server"
	"github.com/taolicus/asteroids/web"
)

func main() {
	defaultPort := os.Getenv("PORT")
	if defaultPort == "" {
		defaultPort = "8080"
	}
	port := flag.String("port", defaultPort, "HTTP listen port")
	flag.Parse()

	mgr := rooms.NewManager()
	// Single server-wide expiry sweeper; it lives for the process
	// lifetime. Individual rooms never own goroutines.
	go mgr.Start(context.Background())

	mux := http.NewServeMux()
	mux.Handle("/api/", server.Handler(mgr))
	mux.Handle("/", http.FileServer(http.FS(web.FS)))

	addr := fmt.Sprintf(":%s", *port)
	log.Printf("serving on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
