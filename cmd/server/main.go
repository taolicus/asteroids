package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/taolicus/asteroids/web"
)

func main() {
	defaultPort := os.Getenv("PORT")
	if defaultPort == "" {
		defaultPort = "8080"
	}
	port := flag.String("port", defaultPort, "HTTP listen port")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(web.FS)))

	addr := fmt.Sprintf(":%s", *port)
	log.Printf("serving on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
