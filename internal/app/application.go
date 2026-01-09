package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	bitbucketcloud "github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/bitbucket-cloud"
	bitbucketdc "github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/bitbucket-dc"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/circuitbreaker"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/claude"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/git"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/logger"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/metrics"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/profiles"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/queue"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/app/handlers"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/app/validator"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/services"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/infrastructure/ratelimit"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// AppState represents the current state of the application
type AppState string

const (
	StateStarting AppState = "starting"
	StateRunning  AppState = "running"
	StateFailed   AppState = "failed"
)

// StartupStatus tracks the current startup progress
type StartupStatus struct {
	State        AppState
	CurrentStep  string
	ErrorMessage string
	mu           sync.RWMutex
}

// Application is the main application struct with all dependencies
type Application struct {
	// Configuration
	config *config.Config

	// Core services
	reviewService *services.ReviewService

	// Ports/Interfaces
	logger           ports.Logger
	vcsClient        ports.VCSClient
	webhookParser    ports.VCSWebhookParser
	gitRepo          ports.GitRepository
	aiReviewer       ports.AIReviewer
	profileProvider  ports.ProfileProvider
	metricsCollector ports.MetricsCollector
	circuitBreaker   ports.CircuitBreaker
	rateLimiter      ports.RateLimiter
	queue            ports.ReviewQueue

	// HTTP server
	server *http.Server

	// Startup status
	startupStatus *StartupStatus

	// Metrics persistence cancel function
	metricsSaveCancel context.CancelFunc
}

// Get returns the current startup status (thread-safe)
func (s *StartupStatus) Get() (interface{}, string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State, s.CurrentStep, s.ErrorMessage
}

// SetStatus updates the startup status (thread-safe)
func (s *StartupStatus) SetStatus(state AppState, step string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
	s.CurrentStep = step
	s.ErrorMessage = errMsg
}

// New creates and initializes a new Application
func New(cfg *config.Config) (*Application, error) {
	// Initialize logger first
	log, err := logger.New(logger.Config{
		Level:             cfg.Logging.Level,
		EnableConsole:     cfg.Logging.EnableConsole,
		EnableFile:        cfg.Logging.EnableFile,
		MaxFileSize:       cfg.Logging.MaxFileSize,
		FileRetentionDays: cfg.Logging.FileRetentionDays,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	log.Info("Initializing application")

	// Initialize metrics collector with persistence config
	metricsCollector := metrics.NewCollector(log, metrics.PersistenceConfig{
		Enabled:      cfg.Metrics.Persistence.Enabled,
		Type:         cfg.Metrics.Persistence.Type,
		Path:         cfg.Metrics.Persistence.Path,
		SaveInterval: cfg.Metrics.Persistence.SaveInterval,
	})

	// Restore metrics from persistent storage
	if err := metricsCollector.Restore(context.Background()); err != nil {
		log.Warn("Failed to restore metrics from storage", "error", err)
	}

	// Initialize circuit breaker with metrics callback
	circuitBreaker := circuitbreaker.NewBreaker(circuitbreaker.Config{
		FailureThreshold: cfg.CircuitBreaker.FailureThreshold,
		ResetTimeout:     time.Duration(cfg.CircuitBreaker.ResetTimeoutMS) * time.Millisecond,
		OnStateChange: func(from, to ports.CircuitState) {
			log.Info("Circuit breaker state changed",
				"from", string(from),
				"to", string(to),
			)
			metricsCollector.IncrementCircuitBreakerTransition(string(from), string(to))
			metricsCollector.RecordCircuitBreakerState(string(to))
		},
	})

	// Initialize VCS client and webhook parser based on platform
	var vcsClient ports.VCSClient
	var webhookParser ports.VCSWebhookParser

	if cfg.Bitbucket.SelfHosted {
		// Bitbucket Data Center / Server
		log.Info("Initializing Bitbucket Data Center adapter")
		vcsClient = bitbucketdc.NewClient(bitbucketdc.Config{
			BaseURL:  cfg.Bitbucket.BaseURL,
			Username: cfg.Bitbucket.User,
			Token:    cfg.Bitbucket.Token,
			Timeout:  30 * time.Second,
		}, log)
		webhookParser = bitbucketdc.NewWebhookParser()
	} else {
		// Bitbucket Cloud
		log.Info("Initializing Bitbucket Cloud adapter")
		vcsClient = bitbucketcloud.NewClient(bitbucketcloud.Config{
			Username: cfg.Bitbucket.User,
			Token:    cfg.Bitbucket.Token,
			Timeout:  30 * time.Second,
		}, log)
		webhookParser = bitbucketcloud.NewWebhookParser()
	}

	// Initialize git repository
	gitRepo := git.NewRepository(git.Config{
		BaseDir: "./projects",
	}, log)

	// Initialize profile provider
	profileProvider := profiles.NewProvider(profiles.Config{
		Directory:       cfg.Profiles.Directory,
		DefaultProfile:  cfg.Profiles.Default,
		ProjectProfiles: cfg.Profiles.Projects,
	}, log)

	// Initialize AI reviewer
	aiReviewer := claude.NewReviewer(claude.Config{
		ModelName: cfg.Claude.Model,
	}, log)

	// Initialize review service
	credentials := ports.NewCredentials(cfg.Bitbucket.User, cfg.Bitbucket.Token)
	reviewService := services.NewReviewService(
		vcsClient,
		gitRepo,
		aiReviewer,
		profileProvider,
		metricsCollector,
		log,
		credentials,
		cfg.Claude.TimeoutMinutes*60, // convert to seconds
	)

	// Initialize rate limiter if enabled
	var rateLimiter ports.RateLimiter
	if cfg.RateLimit.Enabled {
		rateLimiter = initializeRateLimiter(cfg, log)
	}

	// Initialize queue
	reviewQueue := queue.NewQueue(
		queue.Config{
			MaxRetries:  cfg.Queue.MaxRetries,
			QueueSize:   cfg.Queue.MaxSize,
			WorkerCount: cfg.Queue.WorkerCount,
		},
		reviewService,
		circuitBreaker,
		vcsClient,
		log,
		metricsCollector,
	)

	app := &Application{
		config:           cfg,
		reviewService:    reviewService,
		logger:           log,
		vcsClient:        vcsClient,
		webhookParser:    webhookParser,
		gitRepo:          gitRepo,
		aiReviewer:       aiReviewer,
		profileProvider:  profileProvider,
		metricsCollector: metricsCollector,
		circuitBreaker:   circuitBreaker,
		rateLimiter:      rateLimiter,
		queue:            reviewQueue,
		startupStatus: &StartupStatus{
			State:       StateStarting,
			CurrentStep: "Initializing",
		},
	}

	// Initialize HTTP server
	if err := app.initHTTPServer(); err != nil {
		return nil, fmt.Errorf("failed to initialize HTTP server: %w", err)
	}

	log.Info("Application initialized successfully")

	return app, nil
}

// initHTTPServer initializes the HTTP server with routes
func (a *Application) initHTTPServer() error {
	mux := http.NewServeMux()

	// Create webhook handler
	webhookHandler := handlers.NewWebhookHandler(
		a.vcsClient,
		a.webhookParser,
		a.queue,
		a.rateLimiter,
		a.metricsCollector,
		a.logger,
		handlers.WebhookConfig{
			WebhookSecret:      a.config.Bitbucket.WebhookSecret,
			AllowedProjectKeys: a.config.Bitbucket.AllowedProjectKeys,
			TriggeringEvents:   a.config.Bitbucket.Events,
			BitbucketUsername:  a.config.Bitbucket.User,
		},
	)

	// Register routes
	mux.HandleFunc("/webhook/bitbucket", webhookHandler.HandleWebhook)
	mux.HandleFunc("/health", handlers.HandleHealth(a.queue, a.circuitBreaker, a.startupStatus))
	mux.Handle("/metrics", promhttp.Handler())

	a.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", a.config.Server.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return nil
}

// Start starts the application with step-by-step startup:
// 1. Start HTTP server (so /health is available immediately)
// 2. Run validation checks
// 3. Start queue workers and other components
func (a *Application) Start(ctx context.Context) error {
	a.logger.Info("Starting application",
		"port", a.config.Server.Port,
	)

	// Step 1: Start HTTP server
	if err := a.startHTTPServer(); err != nil {
		return err
	}

	// Step 2: Run validation checks
	if err := a.validateStartup(); err != nil {
		a.startupStatus.SetStatus(StateFailed, "Validation failed", err.Error())
		return err
	}

	// Step 3: Start components
	if err := a.startComponents(ctx); err != nil {
		a.startupStatus.SetStatus(StateFailed, "Failed to start components", err.Error())
		return err
	}

	a.logger.Info("Application started successfully - ready to process webhooks")
	return nil
}

// startHTTPServer starts the HTTP server and waits for it to be listening
func (a *Application) startHTTPServer() error {
	a.startupStatus.SetStatus(StateStarting, "Starting HTTP server", "")

	errChan := make(chan error, 1)
	go func() {
		a.logger.Info("Starting HTTP server",
			"address", a.server.Addr,
		)

		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("HTTP server error", "error", err)
			a.startupStatus.SetStatus(StateFailed, "HTTP server failed", err.Error())
			errChan <- err
		}
	}()

	// Wait briefly to catch immediate startup errors (like port already in use)
	select {
	case err := <-errChan:
		return fmt.Errorf("failed to start HTTP server: %w", err)
	case <-time.After(100 * time.Millisecond):
		a.logger.Info("HTTP server started successfully")
		return nil
	}
}

// validateStartup runs all startup validation checks
func (a *Application) validateStartup() error {
	a.startupStatus.SetStatus(StateStarting, "Running validation checks", "")

	validationResult := validator.ValidateStartup(a.config, a.logger, a.vcsClient)
	validationResult.LogResults(a.logger)

	if !validationResult.IsValid() {
		return fmt.Errorf("startup validation failed")
	}

	a.logger.Info("All startup dependency checks passed")
	a.logger.Info("")

	return nil
}

// startComponents initializes queue workers and other components
func (a *Application) startComponents(ctx context.Context) error {
	a.startupStatus.SetStatus(StateStarting, "Initializing queue workers", "")

	// Start queue workers
	a.queue.Start(ctx)
	a.logger.Info("Queue workers started")

	// Start periodic metrics saving if enabled
	if a.config.Metrics.Persistence.Enabled && a.config.Metrics.Persistence.SaveInterval > 0 {
		saveCtx, cancel := context.WithCancel(context.Background())
		a.metricsSaveCancel = cancel
		go a.periodicMetricsSave(saveCtx, a.config.Metrics.Persistence.SaveInterval)
		a.logger.Info("Metrics periodic save started",
			"interval", a.config.Metrics.Persistence.SaveInterval,
		)
	}

	// Mark as fully running
	a.startupStatus.SetStatus(StateRunning, "All components running", "")
	a.logger.Info("Application fully started and ready")

	return nil
}

// periodicMetricsSave saves metrics periodically
func (a *Application) periodicMetricsSave(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.metricsCollector.Save(ctx); err != nil {
				a.logger.Warn("Failed to save metrics", "error", err)
			}
		}
	}
}

// Stop gracefully stops the application
func (a *Application) Stop(ctx context.Context) error {
	a.logger.Info("Stopping application")

	// Stop periodic metrics saving
	if a.metricsSaveCancel != nil {
		a.metricsSaveCancel()
	}

	// Final save of metrics before shutdown
	if err := a.metricsCollector.Save(ctx); err != nil {
		a.logger.Warn("Error saving metrics on shutdown", "error", err)
	}

	// Close metrics collector (closes persister)
	if collector, ok := a.metricsCollector.(*metrics.Collector); ok {
		if err := collector.Close(); err != nil {
			a.logger.Warn("Error closing metrics collector", "error", err)
		}
	}

	// Stop queue
	if err := a.queue.Stop(ctx); err != nil {
		a.logger.Warn("Error stopping queue", "error", err)
	}

	// Stop HTTP server
	if err := a.server.Shutdown(ctx); err != nil {
		a.logger.Warn("Error stopping HTTP server", "error", err)
		return err
	}

	return nil
}

// GetLogger returns the logger instance
func (a *Application) GetLogger() ports.Logger {
	return a.logger
}

// GetQueue returns the queue instance
func (a *Application) GetQueue() ports.ReviewQueue {
	return a.queue
}

// GetCircuitBreaker returns the circuit breaker instance
func (a *Application) GetCircuitBreaker() ports.CircuitBreaker {
	return a.circuitBreaker
}

// GetVCSClient returns the VCS client instance
func (a *Application) GetVCSClient() ports.VCSClient {
	return a.vcsClient
}

// initializeRateLimiter creates a rate limiter based on configuration
func initializeRateLimiter(cfg *config.Config, log ports.Logger) ports.RateLimiter {
	requestsPerMinute := cfg.RateLimit.RequestsPerMinute
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60 // default
	}

	log.Info("Rate limiter initialized",
		"requests_per_minute", requestsPerMinute,
	)

	return ratelimit.PerMinute(requestsPerMinute)
}
