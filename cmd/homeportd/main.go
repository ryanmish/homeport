package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/crypto/bcrypt"

	"github.com/gethomeport/homeport/internal/api"
	"github.com/gethomeport/homeport/internal/config"
	"github.com/gethomeport/homeport/internal/store"
)

func main() {
	// Check for subcommands first
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "hash-password":
			hashPassword()
			return
		case "generate-password":
			generatePassword()
			return
		}
	}

	// Flags for server mode
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

	// Load settings from environment
	if externalURL := os.Getenv("HOMEPORT_EXTERNAL_URL"); externalURL != "" {
		cfg.ExternalURL = externalURL
	}
	if codeServerHost := os.Getenv("HOMEPORT_CODE_SERVER_HOST"); codeServerHost != "" {
		cfg.CodeServerHost = codeServerHost
	}
	cfg.PasswordHash = os.Getenv("HOMEPORT_PASSWORD_HASH")
	cfg.CookieSecret = os.Getenv("HOMEPORT_COOKIE_SECRET")

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
	log.Printf("  Auth enabled: %v", cfg.PasswordHash != "")

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// hashPassword reads a password from stdin and outputs a bcrypt hash
func hashPassword() {
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading password:", err)
		os.Exit(1)
	}
	password = strings.TrimSpace(password)

	if len(password) < 8 {
		fmt.Fprintln(os.Stderr, "Password must be at least 8 characters")
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error hashing password:", err)
		os.Exit(1)
	}

	fmt.Println(string(hash))
}

// generatePassword creates a random password and outputs both the password and its hash
func generatePassword() {
	// Generate 18 random bytes = 24 base64 chars, take first 20 for readability
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		fmt.Fprintln(os.Stderr, "Error generating password:", err)
		os.Exit(1)
	}
	password := base64.URLEncoding.EncodeToString(b)[:20]

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error hashing password:", err)
		os.Exit(1)
	}

	// Output format: password on first line, hash on second line
	fmt.Println(password)
	fmt.Println(string(hash))
}
