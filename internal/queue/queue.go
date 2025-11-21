package queue

import (
	"sync"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/bitbucket"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/circuitbreaker"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/claude"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
	"github.com/asmild/bitbucket-pr-reviewer-bot/pkg/models"
)

// Item represents an item in the processing queue
type Item struct {
	PRData    *models.PRData
	EventType string
	QueuedAt  int64
}

// Queue manages sequential processing of pull requests
type Queue struct {
	cfg            *config.Config
	circuitBreaker *circuitbreaker.CircuitBreaker
	bbClient       *bitbucket.Client
	items          []*Item
	mu             sync.Mutex
	processing     bool
	itemCh         chan *Item
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// New creates a new queue
func New(cfg *config.Config, cb *circuitbreaker.CircuitBreaker, bbClient *bitbucket.Client) *Queue {
	q := &Queue{
		cfg:            cfg,
		circuitBreaker: cb,
		bbClient:       bbClient,
		items:          make([]*Item, 0),
		itemCh:         make(chan *Item, 100),
		stopCh:         make(chan struct{}),
	}

	// Start worker
	q.wg.Add(1)
	go q.worker()

	return q
}

// Enqueue adds a new item to the queue
func (q *Queue) Enqueue(prData *models.PRData, eventType string) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	item := &Item{
		PRData:    prData,
		EventType: eventType,
		QueuedAt:  time.Now().Unix(),
	}

	q.items = append(q.items, item)
	position := len(q.items)

	logger.WithFields(map[string]interface{}{
		"repository":    prData.Repository,
		"prTitle":       prData.Title,
		"queuePosition": position,
	}).Info("PR added to queue")

	// Send to channel
	select {
	case q.itemCh <- item:
	default:
		logger.Warn("Queue channel full, item will be processed later")
	}

	return position
}

// worker processes items from the queue
func (q *Queue) worker() {
	defer q.wg.Done()

	for {
		select {
		case item := <-q.itemCh:
			q.processItem(item)
		case <-q.stopCh:
			return
		}
	}
}

// processItem processes a single queue item
func (q *Queue) processItem(item *Item) {
	q.mu.Lock()
	q.processing = true
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		q.processing = false
		// Remove item from queue
		if len(q.items) > 0 {
			q.items = q.items[1:]
		}
		q.mu.Unlock()
	}()

	logger.WithFields(map[string]interface{}{
		"repository": item.PRData.Repository,
		"prTitle":    item.PRData.Title,
		"eventType":  item.EventType,
	}).Info("Processing PR from queue")

	// Add "thinking_face" emoji when review starts
	q.bbClient.ReplaceReaction(item.PRData, "thinking_face")

	// Check circuit breaker
	if q.circuitBreaker.GetState() == circuitbreaker.StateOpen {
		logger.Warn("Circuit breaker is open, skipping PR processing")
		q.bbClient.ReplaceReaction(item.PRData, "x") // X emoji for failure
		return
	}

	// Process with circuit breaker protection
	err := q.circuitBreaker.Call(func() error {
		return claude.ProcessPullRequest(q.cfg, item.PRData)
	})

	if err != nil {
		// Add "x" or "thumbs_down" emoji on failure
		q.bbClient.ReplaceReaction(item.PRData, "x")

		if err == circuitbreaker.ErrCircuitOpen {
			logger.Error("Circuit breaker opened, stopping PR processing temporarily")
		} else {
			logger.WithFields(map[string]interface{}{
				"repository": item.PRData.Repository,
				"error":      err.Error(),
			}).Error("Failed to process PR")
		}
	} else {
		// Add "thumbs_up" emoji on success
		q.bbClient.ReplaceReaction(item.PRData, "+1")

		logger.WithFields(map[string]interface{}{
			"repository": item.PRData.Repository,
			"prTitle":    item.PRData.Title,
		}).Info("PR processed successfully")
	}
}

// GetQueueSize returns the current queue size
func (q *Queue) GetQueueSize() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// IsProcessing returns whether the queue is currently processing
func (q *Queue) IsProcessing() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.processing
}

// Shutdown gracefully shuts down the queue
func (q *Queue) Shutdown() {
	logger.Info("Shutting down queue...")
	close(q.stopCh)
	q.wg.Wait()
	logger.Info("Queue shut down")
}
