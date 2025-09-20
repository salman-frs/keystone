package github

import (
	"context"
	"sync"
	"time"
)

// Priority levels for request queue
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// Request represents a queued API request
type Request struct {
	ID       string
	Priority Priority
	Fn       func(ctx context.Context) error
	Result   chan error
	Created  time.Time
}

// Queue implements a priority-based request queue for batch operations
type Queue struct {
	client        *Client
	queues        map[Priority]chan *Request
	workers       int
	shutdown      chan struct{}
	wg            sync.WaitGroup
	maxRetries    int
	retryDelay    time.Duration
	batchSize     int
	batchInterval time.Duration
}

// QueueConfig holds queue configuration
type QueueConfig struct {
	Workers       int
	MaxRetries    int
	RetryDelay    time.Duration
	BatchSize     int
	BatchInterval time.Duration
	QueueSize     int
}

// DefaultQueueConfig returns default queue configuration
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		Workers:       5,
		MaxRetries:    3,
		RetryDelay:    5 * time.Second,
		BatchSize:     10,
		BatchInterval: 1 * time.Second,
		QueueSize:     1000,
	}
}

// NewQueue creates a new request queue
func NewQueue(client *Client, config QueueConfig) *Queue {
	q := &Queue{
		client:        client,
		queues:        make(map[Priority]chan *Request),
		workers:       config.Workers,
		shutdown:      make(chan struct{}),
		maxRetries:    config.MaxRetries,
		retryDelay:    config.RetryDelay,
		batchSize:     config.BatchSize,
		batchInterval: config.BatchInterval,
	}

	// Initialize priority queues
	q.queues[PriorityCritical] = make(chan *Request, config.QueueSize/4)
	q.queues[PriorityHigh] = make(chan *Request, config.QueueSize/4)
	q.queues[PriorityNormal] = make(chan *Request, config.QueueSize/2)
	q.queues[PriorityLow] = make(chan *Request, config.QueueSize/4)

	return q
}

// Start begins processing requests from the queue
func (q *Queue) Start() {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

// Stop gracefully shuts down the queue
func (q *Queue) Stop() {
	close(q.shutdown)
	
	// Close all queues to signal workers to stop
	for _, queue := range q.queues {
		close(queue)
	}
	
	q.wg.Wait()
}

// Enqueue adds a request to the appropriate priority queue
func (q *Queue) Enqueue(ctx context.Context, id string, priority Priority, fn func(ctx context.Context) error) <-chan error {
	req := &Request{
		ID:       id,
		Priority: priority,
		Fn:       fn,
		Result:   make(chan error, 1),
		Created:  time.Now(),
	}

	select {
	case q.queues[priority] <- req:
		return req.Result
	case <-ctx.Done():
		req.Result <- ctx.Err()
		return req.Result
	case <-q.shutdown:
		req.Result <- ErrQueueShutdown
		return req.Result
	}
}

// ErrQueueShutdown is returned when the queue is shutting down
var ErrQueueShutdown = fmt.Errorf("queue is shutting down")

// worker processes requests from priority queues
func (q *Queue) worker(id int) {
	defer q.wg.Done()

	batch := make([]*Request, 0, q.batchSize)
	ticker := time.NewTicker(q.batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.shutdown:
			// Process remaining batch before shutdown
			if len(batch) > 0 {
				q.processBatch(batch)
			}
			return

		case <-ticker.C:
			// Process batch on interval
			if len(batch) > 0 {
				q.processBatch(batch)
				batch = batch[:0] // Reset batch
			}

		default:
			// Try to get requests from priority queues
			req := q.getNextRequest()
			if req == nil {
				// No requests available, wait a bit
				time.Sleep(100 * time.Millisecond)
				continue
			}

			batch = append(batch, req)

			// Process batch if it's full
			if len(batch) >= q.batchSize {
				q.processBatch(batch)
				batch = batch[:0] // Reset batch
			}
		}
	}
}

// getNextRequest gets the next request from priority queues
func (q *Queue) getNextRequest() *Request {
	// Check queues in priority order
	priorities := []Priority{PriorityCritical, PriorityHigh, PriorityNormal, PriorityLow}
	
	for _, priority := range priorities {
		select {
		case req, ok := <-q.queues[priority]:
			if !ok {
				continue // Queue is closed
			}
			return req
		default:
			// No request in this priority queue, try next
		}
	}
	
	return nil
}

// processBatch processes a batch of requests
func (q *Queue) processBatch(batch []*Request) {
	for _, req := range batch {
		go q.processRequest(req)
	}
}

// processRequest processes a single request with retries
func (q *Queue) processRequest(req *Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastErr error
	
	for attempt := 0; attempt <= q.maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(q.retryDelay * time.Duration(attempt)):
			case <-ctx.Done():
				req.Result <- ctx.Err()
				return
			}
		}

		// Execute the request function
		lastErr = req.Fn(ctx)
		if lastErr == nil {
			req.Result <- nil
			return
		}

		// Check if error is retryable
		if !q.isRetryableError(lastErr) {
			break
		}
	}

	req.Result <- lastErr
}

// isRetryableError determines if an error is retryable
func (q *Queue) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Retry on circuit breaker errors and rate limit errors
	return err == circuit.ErrCircuitOpen || 
		   err == circuit.ErrTooManyCalls ||
		   err == ErrRequestTimeout ||
		   err.Error() == "rate limit exceeded"
}

// Stats returns queue statistics
type QueueStats struct {
	QueueLengths map[Priority]int
	WorkerCount  int
	TotalQueued  int
}

// Stats returns current queue statistics
func (q *Queue) Stats() QueueStats {
	stats := QueueStats{
		QueueLengths: make(map[Priority]int),
		WorkerCount:  q.workers,
	}

	for priority, queue := range q.queues {
		length := len(queue)
		stats.QueueLengths[priority] = length
		stats.TotalQueued += length
	}

	return stats
}