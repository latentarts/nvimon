package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/prods/nvimon/internal/collector"
	"github.com/prods/nvimon/internal/config"
	"github.com/prods/nvimon/internal/transport/httpapi"
)

func main() {
	configPath := flag.String("config", config.DefaultPath(), "path to config file")
	bindAddress := flag.String("bind-address", "", "override agent bind address")
	authToken := flag.String("auth-token", "", "override agent auth token")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if *bindAddress != "" {
		cfg.Agent.BindAddress = *bindAddress
	}
	if *authToken != "" {
		cfg.Agent.AuthToken = *authToken
	}

	server := httpapi.NewServer(
		collector.NewLocalCollector(cfg.RefreshInterval),
		cfg.Agent.AuthToken,
	)

	fmt.Fprintf(os.Stdout, "nvimon-agent listening on http://%s\n", cfg.Agent.BindAddress)
	httpServer := &http.Server{
		Addr:    cfg.Agent.BindAddress,
		Handler: server.Handler(),
		ConnState: func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				log.Printf("client connected: %s", conn.RemoteAddr())
			case http.StateClosed, http.StateHijacked:
				log.Printf("client disconnected: %s", conn.RemoteAddr())
			}
		},
	}

	if err := httpServer.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}
}
