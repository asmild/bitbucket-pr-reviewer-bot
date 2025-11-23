package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/bitbucket"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/circuitbreaker"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/claude"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/git"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/logger"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/metrics"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/profiles"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/queue"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/app/handlers"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/services"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/infrastructure/ratelimit"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector(log)

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

	// Initialize VCS client and webhook parser
	vcsClient := bitbucket.NewClient(bitbucket.Config{
		Username: cfg.Bitbucket.User,
		Token:    cfg.Bitbucket.Token,
		Timeout:  30 * time.Second,
	}, log)

	webhookParser := bitbucket.NewWebhookParser()

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
			MaxRetries: cfg.Queue.MaxRetries,
			QueueSize:  cfg.Queue.MaxSize,
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
			TriggerKeyword:     a.config.Bitbucket.TriggerKeyword,
			EventType:          a.config.Bitbucket.EventType,
			BitbucketUsername:  a.config.Bitbucket.User,
		},
	)

	// Register routes
	mux.HandleFunc("/webhook", webhookHandler.HandleWebhook)
	mux.HandleFunc("/health", handlers.HandleHealth(a.queue, a.circuitBreaker))
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

// Start starts the application
func (a *Application) Start(ctx context.Context) error {
	a.logger.Info("Starting application",
		"port", a.config.Server.Port,
	)

	// Start queue
	a.queue.Start(ctx)

	// Start HTTP server in goroutine
	go func() {
		a.logger.Info("Starting HTTP server",
			"address", a.server.Addr,
		)

		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the application
func (a *Application) Stop(ctx context.Context) error {
	a.logger.Info("Stopping application")

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
