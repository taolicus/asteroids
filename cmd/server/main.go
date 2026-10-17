package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/taolicus/asteroids/internal/game"
	"github.com/taolicus/asteroids/internal/identity"
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

	// ADDR is a full host:port, used by the production unit. It takes
	// precedence over the port flag.
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = fmt.Sprintf(":%s", *port)
	}

	// LOBBY_KEY signs player credentials. It must be stable across
	// restarts or existing owners lose their lobbies (their links still
	// resolve, but as guest-only). Without it, local development gets a
	// per-process key and identities do not survive a restart.
	key := os.Getenv("LOBBY_KEY")
	if key == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			log.Fatalf("generate fallback LOBBY_KEY: %v", err)
		}
		key = string(buf)
		log.Print("LOBBY_KEY is not set; using a random key. Lobby ownership will not survive a restart.")
	}

	mgr := rooms.NewManager()
	// Single server-wide reclaim sweeper; it lives for the process
	// lifetime. Individual lobbies never own goroutines.
	go mgr.Start(context.Background())

	// Per-room sims, reclaimed on the same idle grace as the lobby so the
	// two lifecycles agree. The input endpoint starts a sim on first use.
	sessions := game.NewSessions(rooms.IdleGrace)
	go sessions.StartSweeper(context.Background())

	mux := http.NewServeMux()
	mux.Handle("/api/", server.Handler(mgr, identity.NewIssuer([]byte(key)), sessions))
	mux.Handle("/", http.FileServer(http.FS(web.FS)))

	log.Printf("serving on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
