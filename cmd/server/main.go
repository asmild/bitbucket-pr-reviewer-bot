package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/app"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
)

func main() {
	log.Println("Starting Bitbucket PR Reviewer Bot...")

	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	var cfg *config.Config
	cfg = config.Load(configPath)

	// Create application
	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	logger := application.GetLogger()

	// Start application (HTTP server, validation, workers - all handled internally)
	ctx := context.Background()
	if err := application.Start(ctx); err != nil {
		logger.Fatal("Failed to start application")
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutdown signal received, gracefully stopping...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop application
	if err := application.Stop(shutdownCtx); err != nil {
		logger.Error("Error during shutdown")
		os.Exit(1)
	}

	logger.Info("Application stopped successfully")
}
