package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/app"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/app/validator"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
)

func main() {
	log.Println("Starting Bitbucket PR Reviewer Bot...")

	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	var cfg *config.Config
	if configPath != "" {
		cfg = config.LoadWithPath(configPath)
	} else {
		cfg = config.Load()
	}

	// Create application
	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	logger := application.GetLogger()

	// Validate startup dependencies
	validationResult := validator.ValidateStartup(cfg, logger)
	validationResult.LogResults(logger)

	if !validationResult.IsValid() {
		logger.Fatal("Startup validation failed - exiting")
	}

	logger.Info("All startup dependency checks passed")
	logger.Info("")

	// Start application
	ctx := context.Background()
	if err := application.Start(ctx); err != nil {
		logger.Fatal("Failed to start application")
	}

	logger.Info("Application started successfully - ready to process webhooks")

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
