package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/bitbucket"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/circuitbreaker"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	httputil "github.com/asmild/bitbucket-pr-reviewer-bot/internal/http"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/metrics"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/queue"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/startup"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	cfg            *config.Config
	prQueue        *queue.Queue
	circuitBreaker *circuitbreaker.CircuitBreaker
	bbClient       *bitbucket.Client
)

func main() {
	// Load configuration
	cfg = config.Load()

	// Initialize logger
	if err := logger.Init(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Starting Bitbucket Auto-PR reviewer Bot's service...")
	logger.Info("Checking startup dependencies...")

	// Validate all startup dependencies
	validationResult := startup.ValidateDependencies(cfg)
	if !validationResult.IsValid() {
		validationResult.LogErrors()
		os.Exit(1)
	}

	logger.Info("All startup dependencies satisfied")

	// Initialize metrics persistence
	if err := metrics.InitPersistence(cfg); err != nil {
		logger.Fatalf("Failed to initialize metrics persistence: %v", err)
	}

	// Initialize circuit breaker
	cbResetTimeout := time.Duration(cfg.CircuitBreaker.ResetTimeoutMS) * time.Millisecond
	circuitBreaker = circuitbreaker.New(cfg.CircuitBreaker.FailureThreshold, cbResetTimeout)
	logger.Infof("Circuit breaker initialized (threshold: %d, reset timeout: %v)", cfg.CircuitBreaker.FailureThreshold, cbResetTimeout)

	// Initialize Bitbucket client
	bbClient = bitbucket.NewClient(cfg)

	// Initialize queue
	prQueue = queue.New(cfg, circuitBreaker, bbClient)

	// Set up HTTP server
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", healthHandler)

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Webhook endpoint
	mux.HandleFunc("/webhook/bitbucket/pr", webhookHandler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Infof("Server starting on port %d", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	logger.Info("PR Automation Service is running")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutdown signal received, gracefully shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Errorf("Server shutdown error: %v", err)
	}

	// Shutdown queue
	prQueue.Shutdown()

	// Shutdown metrics persistence
	metrics.ShutdownPersistence()

	logger.Info("Service shut down successfully")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","message":"PR Automation service is running"}`))
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read and log raw body for debugging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Warnf("Failed to read request body: %v", err)
		httputil.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Failed to read request body")
		return
	}
	logger.Debugf("Received webhook payload: %s", string(bodyBytes))

	// Get event type and signature headers
	eventType := r.Header.Get("X-Event-Key")
	signature := r.Header.Get("X-Hub-Signature")
	logger.Debugf("Received webhook event type: %s", eventType)
	logger.Debugf("Webhook signature header: %s", signature)

	if eventType == "" {
		httputil.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Missing X-Event-Key header")
		return
	}

	// Determine expected event type based on configuration
	var expectedEventType string
	if cfg.Bitbucket.EventType == "comment_added" {
		expectedEventType = bitbucket.EventTypeCommentAdded
	} else {
		expectedEventType = bitbucket.EventTypePROpened
	}

	// Only process configured event type
	if eventType != expectedEventType {
		logger.Debugf("Ignoring event: %s (expecting: %s)", eventType, expectedEventType)
		httputil.WriteSuccessResponse(w, "", 0)
		return
	}

	// Parse payload (reuse bodyBytes we already read)
	payload, payloadBytes, err := bitbucket.ParsePayload(bytes.NewReader(bodyBytes))
	if err != nil {
		logger.Warnf("Failed to parse webhook payload: %v", err)
		httputil.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Invalid payload structure: %v", err))
		return
	}

	// Validate signature
	if err := bitbucket.ValidateSignature(cfg.Bitbucket.WebhookSecret, payloadBytes, signature); err != nil {
		logger.Warnf("Webhook signature validation failed: %v", err)
		httputil.WriteErrorResponse(w, http.StatusUnauthorized, "Unauthorized", err.Error())
		return
	}
	logger.Debugf("Webhook signature validation passed")

	// Validate comment trigger (only for comment_added events)
	if cfg.Bitbucket.EventType == "comment_added" {
		if err := bitbucket.ValidateCommentTrigger(cfg, payload); err != nil {
			logger.Debugf("Comment trigger validation failed: %v", err)
			httputil.WriteSuccessResponse(w, "", 0)
			return
		}
	}

	// Validate project
	if err := bitbucket.ValidateProject(cfg, payload); err != nil {
		logger.Warnf("Project validation failed: %v", err)
		httputil.WriteErrorResponse(w, http.StatusForbidden, "Forbidden", err.Error())
		return
	}

	// Extract PR data
	prData, err := bitbucket.ExtractPRData(payload)
	if err != nil {
		logger.Warnf("Failed to extract PR data: %v", err)
		httputil.WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Failed to extract PR data: %v", err))
		return
	}

	// Set Bitbucket base URL from the clone URL
	baseURL := bitbucket.ExtractBaseURL(prData.RepoCloneURL)
	bbClient.SetBaseURL(baseURL)

	// Add "eyes" emoji to acknowledge the trigger (only for comment events with comment ID)
	if cfg.Bitbucket.EventType == "comment_added" && prData.CommentID > 0 {
		bbClient.ReplaceReaction(prData, "eyes")
	}

	// Record metrics based on event type
	if eventType == bitbucket.EventTypePROpened {
		metrics.RecordPRCreated(prData.Repository)
	} else {
		metrics.RecordPRUpdated(prData.Repository)
	}

	// Add to queue
	position := prQueue.Enqueue(prData, eventType)

	logger.WithFields(map[string]interface{}{
		"repository":    prData.Repository,
		"prTitle":       prData.Title,
		"eventType":     eventType,
		"queuePosition": position,
	}).Info("Webhook received successfully")

	// Send success response
	httputil.WriteSuccessResponse(w, prData.Title, position)
}
