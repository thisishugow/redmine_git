package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"colosscious.com/gitfetcher/config"
	"colosscious.com/gitfetcher/fetcher"
	"colosscious.com/gitfetcher/scheduler"
	"colosscious.com/gitfetcher/web"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	version    = "1.0.0"
)

func main() {
	flag.Parse()

	log.Printf("GitFetcher v%s starting...", version)

	// Load initial configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Loaded config from %s", *configPath)

	// Initialize components
	gitFetcher := fetcher.NewGitFetcher(cfg.SSHKeyPath, cfg.LogPath)
	sched := scheduler.NewScheduler(gitFetcher)
	sched.LoadConfig(cfg)

	// Setup HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	handler := web.NewHandler(sched, *configPath)
	handler.SetupRoutes(router)

	// Start config file watcher for hot reload
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create file watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Add(*configPath); err != nil {
		log.Printf("Warning: Failed to watch config file: %v", err)
	} else {
		go watchConfigFile(watcher, sched)
	}

	// Start HTTP server in background
	go func() {
		addr := fmt.Sprintf(":%d", cfg.HTTPPort)
		log.Printf("Starting HTTP server on %s", addr)
		if err := router.Run(addr); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	log.Println("GitFetcher is running. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	sched.Stop()
	log.Println("GitFetcher stopped")
}

// watchConfigFile monitors config file changes and reloads
func watchConfigFile(watcher *fsnotify.Watcher, sched *scheduler.Scheduler) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Printf("Config file changed, reloading...")

				cfg, err := config.LoadConfig(*configPath)
				if err != nil {
					log.Printf("Failed to reload config: %v", err)
					continue
				}

				sched.LoadConfig(cfg)
				log.Println("Config reloaded successfully")
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}
