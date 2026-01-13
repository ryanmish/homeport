package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gethomeport/homeport/internal/api"
	"github.com/gethomeport/homeport/internal/config"
	"github.com/gethomeport/homeport/internal/store"
)

func main() {
	// Flags
	devMode := flag.Bool("dev", false, "Run in development mode (uses local paths, lsof on Mac)")
	configPath := flag.String("config", "", "Path to config file")
	listenAddr := flag.String("listen", "", "Override listen address (e.g., :8080)")
	flag.Parse()

	// Load config
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.Load(*configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	} else if *devMode {
		cfg = config.DefaultDev()
	} else {
		cfg = config.Default()
	}

	cfg.DevMode = *devMode

	if *listenAddr != "" {
		cfg.ListenAddr = *listenAddr
	}

	// Ensure directories exist
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Initialize store
	st, err := store.New(cfg.DBPath())
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	defer st.Close()

	// Create and start server
	server := api.NewServer(cfg, st)

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		server.Stop()
		os.Exit(0)
	}()

	// Start server
	log.Printf("Homeport daemon starting...")
	log.Printf("  Dev mode: %v", cfg.DevMode)
	log.Printf("  Listen: %s", cfg.ListenAddr)
	log.Printf("  Repos: %s", cfg.ReposDir)
	log.Printf("  Data: %s", cfg.DataDir)
	log.Printf("  Port range: %d-%d", cfg.PortRangeMin, cfg.PortRangeMax)

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
