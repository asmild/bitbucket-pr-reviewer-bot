package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/services"
)

// Queue implements ports.ReviewQueue
type Queue struct {
	reviewService  *services.ReviewService
	circuitBreaker ports.CircuitBreaker
	vcsClient      ports.VCSClient
	logger         ports.Logger
	metrics        ports.MetricsCollector

	// Queue state
	items   []*queueItem
	mu      sync.Mutex
	running bool

	// Channels
	itemCh chan *queueItem
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Configuration
	maxRetries   int
	queueSize    int
	workerCount  int
	processingMu sync.Mutex   // Protects processedItems set
	processing   map[int]bool // Track which PR IDs are being processed
}

// queueItem represents an item in the queue
type queueItem struct {
	pr          *models.PullRequest
	queuedAt    time.Time
	retryCount  int
	lastAttempt time.Time
}

// Config holds queue configuration
type Config struct {
	MaxRetries  int
	QueueSize   int // Maximum number of items in the queue
	WorkerCount int // Number of concurrent workers (default: 1)
}

// updateReaction is a helper method that calls the package-level updateReaction function
func (q *Queue) updateReaction(pr *models.PullRequest, oldEmoji, newEmoji string) {
	if !pr.IsManualTrigger() || pr.CommentID == 0 {
		return
	}

	// Remove old reaction if specified
	if oldEmoji != "" {
		if err := q.vcsClient.RemoveCommentReaction(context.Background(), pr.ProjectKey, pr.RepositorySlug, pr.ID, pr.CommentID, oldEmoji); err != nil {
			q.logger.Debug("Failed to remove old reaction (might not exist)",
				"old_emoji", oldEmoji,
				"comment_id", pr.CommentID,
			)
		}
	}

	// Add new reaction
	if err := q.vcsClient.AddCommentReaction(context.Background(), pr.ProjectKey, pr.RepositorySlug, pr.ID, pr.CommentID, newEmoji); err != nil {
		q.logger.Warn("Failed to add reaction",
			"emoji", newEmoji,
			"pr_id", pr.ID,
			"error", err,
		)
	}
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
	if cfg.WorkerCount == 0 {
		cfg.WorkerCount = 1
	}

	return &Queue{
		reviewService:  reviewService,
		circuitBreaker: circuitBreaker,
		vcsClient:      vcsClient,
		logger:         logger,
		metrics:        metrics,
		items:          make([]*queueItem, 0),
		itemCh:         make(chan *queueItem, cfg.QueueSize),
		stopCh:         make(chan struct{}),
		maxRetries:     cfg.MaxRetries,
		queueSize:      cfg.QueueSize,
		workerCount:    cfg.WorkerCount,
		processing:     make(map[int]bool),
		running:        false,
	}
}

// Enqueue adds a pull request to the review queue
func (q *Queue) Enqueue(ctx context.Context, pr *models.PullRequest) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Add "queued" reaction if manual trigger
	q.updateReaction(pr, "", "QUEUED")

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

		// Add failed reaction if manual trigger
		q.updateReaction(pr, "", "FAILED")

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

	// Send to channel (blocking, but should never block due to queue size check above)
	select {
	case q.itemCh <- item:
		// Successfully sent to channel
	case <-ctx.Done():
		// Context cancelled, remove item from queue and return error
		q.items = q.items[:len(q.items)-1]
		q.metrics.ObserveQueueSize(len(q.items))
		return 0, ctx.Err()
	}

	return position, nil
}

// Start starts the queue workers
func (q *Queue) Start(ctx context.Context) {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}
	q.running = true
	q.mu.Unlock()

	q.logger.Info("Starting queue workers",
		"worker_count", q.workerCount,
	)

	// Start multiple workers
	for i := 0; i < q.workerCount; i++ {
		workerID := i + 1
		q.wg.Add(1)
		go q.worker(ctx, workerID)
	}
}

// Stop immediately stops the queue workers
// Items in progress are abandoned - reviews can be re-run safely
func (q *Queue) Stop(ctx context.Context) error {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return nil
	}
	q.running = false
	q.mu.Unlock()

	q.logger.Info("Stopping queue workers immediately",
		"worker_count", q.workerCount,
	)

	// Close stopCh to signal all goroutines to stop
	close(q.stopCh)

	// Wait briefly for goroutines to clean up, but don't block shutdown
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		q.logger.Info("Queue workers stopped cleanly")
	case <-time.After(1 * time.Second):
		q.logger.Warn("Queue workers shutdown timeout (1s) - forcing stop",
			"worker_count", q.workerCount,
		)
		// Goroutines will be killed when process exits
	}

	return nil
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

// GetActiveWorkers returns the number of workers currently processing PRs
func (q *Queue) GetActiveWorkers() int {
	q.processingMu.Lock()
	defer q.processingMu.Unlock()
	return len(q.processing)
}

// worker processes items from the queue
func (q *Queue) worker(ctx context.Context, workerID int) {
	defer q.wg.Done()

	q.logger.Info("Queue worker started",
		"worker_id", workerID,
	)

	for {
		select {
		case item := <-q.itemCh:
			q.processItem(ctx, item, workerID)
		case <-q.stopCh:
			q.logger.Info("Queue worker received stop signal",
				"worker_id", workerID,
			)
			return
		case <-ctx.Done():
			q.logger.Info("Queue worker context cancelled",
				"worker_id", workerID,
			)
			return
		}
	}
}

// processItem processes a single queue item
func (q *Queue) processItem(ctx context.Context, item *queueItem, workerID int) {
	// Mark PR as being processed
	q.processingMu.Lock()
	q.processing[item.pr.ID] = true
	q.processingMu.Unlock()

	defer func() {
		// Recover from panic to prevent worker crash
		if r := recover(); r != nil {
			q.logger.Error("Panic recovered in queue worker",
				"panic", r,
				"pr_id", item.pr.ID,
				"project", item.pr.ProjectKey,
				"repo", item.pr.RepositorySlug,
				"worker_id", workerID,
			)

			// Post failure comment about panic
			comment := fmt.Sprintf("Review failed due to internal error (panic recovered).\n\nPlease try again or contact support if the issue persists.")
			if err := q.vcsClient.PostComment(ctx, item.pr.ProjectKey, item.pr.RepositorySlug, item.pr.ID, comment, item.pr.CommentID); err != nil {
				q.logger.Error("Failed to post panic recovery comment", "error", err)
			}

			q.updateReaction(item.pr, "PROCESSING", "FAILED")
		}

		// Mark PR as no longer being processed
		q.processingMu.Lock()
		delete(q.processing, item.pr.ID)
		q.processingMu.Unlock()

		// Remove item from queue
		q.mu.Lock()
		if len(q.items) > 0 {
			q.items = q.items[1:]
			q.metrics.ObserveQueueSize(len(q.items))
		}
		q.mu.Unlock()
	}()

	item.lastAttempt = time.Now()

	q.logger.Info("Processing PR from queue",
		"worker_id", workerID,
		"project", item.pr.ProjectKey,
		"repo", item.pr.RepositoryID,
		"pr_id", item.pr.ID,
		"retry_count", item.retryCount,
	)

	q.updateReaction(item.pr, "QUEUED", "PROCESSING")

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
		q.updateReaction(item.pr, "PROCESSING", "SUCCESS")
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
		// Max retries reached or non-retryable error
		q.logger.Warn("Max retries reached or non-retryable error",
			"pr_id", item.pr.ID,
			"retry_count", item.retryCount,
		)

		q.updateReaction(item.pr, "PROCESSING", "FAILED")
		q.postFailureComment(ctx, item, err)
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

	// Track goroutine and use stoppable timer
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()

		timer := time.NewTimer(backoffDelay)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Backoff completed, try to requeue
			select {
			case q.itemCh <- item:
				q.logger.Debug("Item requeued successfully", "pr_id", item.pr.ID)
			case <-q.stopCh:
				q.logger.Debug("Queue stopped while requeueing, item dropped", "pr_id", item.pr.ID)
			}
		case <-q.stopCh:
			// Immediate shutdown - drop the item
			q.logger.Debug("Queue stopped during backoff, item dropped", "pr_id", item.pr.ID)
		}
	}()
}

// postFailureComment posts a failure comment on the PR
func (q *Queue) postFailureComment(ctx context.Context, item *queueItem, err error) {
	errorCode := errors.GetCode(err)
	comment := fmt.Sprintf("Review failed after %d retries.\n\nError: %s", item.retryCount, string(errorCode))

	q.logger.Info("Posting failure comment",
		"project", item.pr.ProjectKey,
		"pr_id", item.pr.ID,
		"is_reply", item.pr.IsManualTrigger(),
	)

	// If manually triggered, CommentID > 0 and will post as reply, otherwise posts as regular comment
	postErr := q.vcsClient.PostComment(ctx, item.pr.ProjectKey, item.pr.RepositorySlug, item.pr.ID, comment, item.pr.CommentID)

	if postErr != nil {
		q.logger.Error("Failed to post failure comment",
			"pr_id", item.pr.ID,
			"error", postErr,
		)
	}
}
