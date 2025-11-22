package queue

import (
	"context"
	"sync"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/services"
)

// Queue implements ports.ReviewQueue with Dead Letter Queue support
type Queue struct {
	reviewService  *services.ReviewService
	circuitBreaker ports.CircuitBreaker
	vcsClient      ports.VCSClient
	logger         ports.Logger
	metrics        ports.MetricsCollector

	// Queue state
	items      []*queueItem
	dlqItems   []*dlqItem
	mu         sync.Mutex
	processing bool
	running    bool

	// Channels
	itemCh chan *queueItem
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Configuration
	maxRetries int
	queueSize  int
}

// queueItem represents an item in the queue
type queueItem struct {
	pr          *models.PullRequest
	queuedAt    time.Time
	retryCount  int
	lastAttempt time.Time
}

// dlqItem represents an item in the dead letter queue
type dlqItem struct {
	pr         *models.PullRequest
	queuedAt   time.Time
	failedAt   time.Time
	lastError  error
	retryCount int
}

// Config holds queue configuration
type Config struct {
	MaxRetries int
	QueueSize  int // Maximum number of items in the queue
}

// NewQueue creates a new review queue
func NewQueue(
	cfg Config,
	reviewService *services.ReviewService,
	circuitBreaker ports.CircuitBreaker,
	vcsClient ports.VCSClient,
	logger ports.Logger,
	metrics ports.MetricsCollector,
) *Queue {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.QueueSize == 0 {
		cfg.QueueSize = 100
	}

	return &Queue{
		reviewService:  reviewService,
		circuitBreaker: circuitBreaker,
		vcsClient:      vcsClient,
		logger:         logger,
		metrics:        metrics,
		items:          make([]*queueItem, 0),
		dlqItems:       make([]*dlqItem, 0),
		itemCh:         make(chan *queueItem, cfg.QueueSize),
		stopCh:         make(chan struct{}),
		maxRetries:     cfg.MaxRetries,
		queueSize:      cfg.QueueSize,
		running:        false,
	}
}

// Enqueue adds a pull request to the review queue
func (q *Queue) Enqueue(ctx context.Context, pr *models.PullRequest) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.running {
		return 0, errors.ErrQueueClosed
	}

	// Check if queue is full
	if len(q.items) >= q.queueSize {
		q.logger.Warn("Queue is full, rejecting PR",
			"project", pr.ProjectKey,
			"pr_id", pr.ID,
			"queue_size", len(q.items),
			"max_size", q.queueSize,
		)

		// Add emoji reaction if manual trigger
		if pr.IsManualTrigger() {
			go q.addReactionAsync(ctx, pr, "CONFUSED")
		}

		return 0, errors.ErrQueueFull
	}

	item := &queueItem{
		pr:         pr,
		queuedAt:   time.Now(),
		retryCount: 0,
	}

	q.items = append(q.items, item)
	position := len(q.items)

	q.logger.Info("PR added to queue",
		"project", pr.ProjectKey,
		"repo", pr.RepositoryID,
		"pr_id", pr.ID,
		"position", position,
	)

	// Update metrics
	q.metrics.ObserveQueueSize(len(q.items))

	// Send to channel (non-blocking)
	select {
	case q.itemCh <- item:
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		q.logger.Warn("Queue channel full, item will be processed when space available")
	}

	return position, nil
}

// addReactionAsync adds an emoji reaction asynchronously (fire and forget)
func (q *Queue) addReactionAsync(ctx context.Context, pr *models.PullRequest, emoji string) {
	if pr.CommentID == 0 {
		return
	}

	if err := q.vcsClient.AddCommentReaction(ctx, pr.ProjectKey, pr.RepositoryID, pr.ID, pr.CommentID, emoji); err != nil {
		q.logger.Warn("Failed to add queue full reaction",
			"pr_id", pr.ID,
			"error", err,
		)
	}
}

// Start starts the queue worker
func (q *Queue) Start(ctx context.Context) {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}
	q.running = true
	q.mu.Unlock()

	q.logger.Info("Starting queue worker")

	q.wg.Add(1)
	go q.worker(ctx)
}

// Stop gracefully stops the queue worker
func (q *Queue) Stop(ctx context.Context) error {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return nil
	}
	q.running = false
	q.mu.Unlock()

	q.logger.Info("Stopping queue worker")

	close(q.stopCh)

	// Wait for worker to finish with timeout
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		q.logger.Info("Queue worker stopped gracefully")
		return nil
	case <-ctx.Done():
		q.logger.Warn("Queue worker stop timed out")
		return ctx.Err()
	}
}

// GetSize returns the current queue size
func (q *Queue) GetSize() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// IsRunning returns true if the queue is running
func (q *Queue) IsRunning() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.running
}

// GetDLQSize returns the current dead letter queue size
func (q *Queue) GetDLQSize() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.dlqItems)
}

// GetDLQItems returns a copy of dead letter queue items
func (q *Queue) GetDLQItems() []DLQItem {
	q.mu.Lock()
	defer q.mu.Unlock()

	items := make([]DLQItem, len(q.dlqItems))
	for i, item := range q.dlqItems {
		items[i] = DLQItem{
			PR:         item.pr,
			QueuedAt:   item.queuedAt,
			FailedAt:   item.failedAt,
			LastError:  item.lastError.Error(),
			RetryCount: item.retryCount,
		}
	}

	return items
}

// DLQItem represents a dead letter queue item for external use
type DLQItem struct {
	PR         *models.PullRequest
	QueuedAt   time.Time
	FailedAt   time.Time
	LastError  string
	RetryCount int
}

// worker processes items from the queue
func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()

	q.logger.Info("Queue worker started")

	for {
		select {
		case item := <-q.itemCh:
			q.processItem(ctx, item)
		case <-q.stopCh:
			q.logger.Info("Queue worker received stop signal")
			return
		case <-ctx.Done():
			q.logger.Info("Queue worker context cancelled")
			return
		}
	}
}

// processItem processes a single queue item
func (q *Queue) processItem(ctx context.Context, item *queueItem) {
	q.mu.Lock()
	q.processing = true
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		q.processing = false

		// Remove item from queue
		if len(q.items) > 0 {
			q.items = q.items[1:]
			q.metrics.ObserveQueueSize(len(q.items))
		}
		q.mu.Unlock()
	}()

	item.lastAttempt = time.Now()

	q.logger.Info("Processing PR from queue",
		"project", item.pr.ProjectKey,
		"repo", item.pr.RepositoryID,
		"pr_id", item.pr.ID,
		"retry_count", item.retryCount,
	)

	// Check if circuit breaker is open
	if q.circuitBreaker.IsOpen() {
		q.logger.Warn("Circuit breaker is open, requeueing item",
			"pr_id", item.pr.ID,
		)
		q.requeueItem(ctx, item)
		return
	}

	// Process with circuit breaker protection
	var reviewErr error
	err := q.circuitBreaker.Execute(ctx, func() error {
		reviewErr = q.reviewService.ReviewPullRequest(ctx, item.pr)
		return reviewErr
	})

	if err != nil {
		q.handleProcessingError(ctx, item, err)
	} else {
		q.logger.Info("PR processed successfully",
			"project", item.pr.ProjectKey,
			"pr_id", item.pr.ID,
		)
	}
}

// handleProcessingError handles errors during processing
func (q *Queue) handleProcessingError(ctx context.Context, item *queueItem, err error) {
	q.logger.Error("Failed to process PR",
		"project", item.pr.ProjectKey,
		"pr_id", item.pr.ID,
		"retry_count", item.retryCount,
		"error", err,
	)

	// Check if error is retryable
	if errors.IsRetryable(err) && item.retryCount < q.maxRetries {
		q.logger.Info("Error is retryable, requeueing",
			"pr_id", item.pr.ID,
			"retry_count", item.retryCount+1,
		)
		q.requeueItem(ctx, item)
	} else {
		// Move to dead letter queue
		q.logger.Warn("Max retries reached or non-retryable error, moving to DLQ",
			"pr_id", item.pr.ID,
			"retry_count", item.retryCount,
		)
		q.moveToDLQ(item, err)
	}
}

// requeueItem requeues an item for retry
func (q *Queue) requeueItem(ctx context.Context, item *queueItem) {
	item.retryCount++

	// Calculate backoff delay
	backoffDelay := time.Duration(item.retryCount) * 5 * time.Second

	q.logger.Debug("Requeueing item with backoff",
		"pr_id", item.pr.ID,
		"delay", backoffDelay,
	)

	// Requeue after delay
	go func() {
		select {
		case <-time.After(backoffDelay):
			select {
			case q.itemCh <- item:
			case <-ctx.Done():
				q.logger.Warn("Context cancelled while requeueing",
					"pr_id", item.pr.ID,
				)
			}
		case <-ctx.Done():
		}
	}()
}

// moveToDLQ moves an item to the dead letter queue
func (q *Queue) moveToDLQ(item *queueItem, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	dlqItem := &dlqItem{
		pr:         item.pr,
		queuedAt:   item.queuedAt,
		failedAt:   time.Now(),
		lastError:  err,
		retryCount: item.retryCount,
	}

	q.dlqItems = append(q.dlqItems, dlqItem)

	q.logger.Warn("Item moved to DLQ",
		"project", item.pr.ProjectKey,
		"pr_id", item.pr.ID,
		"dlq_size", len(q.dlqItems),
	)
}
