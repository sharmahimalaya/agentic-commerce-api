package webhook

import (
	"acommerce_api_endpoint/models"
	"acommerce_api_endpoint/store"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// Dispatcher manages sending webhook events to registered external subscribers.
// It runs a background worker loop, processes events in parallel using goroutines,
// signs the payloads for security, and handles retries if a subscriber is offline.
type Dispatcher struct {
	EventStore *store.EventStore
	eventChan  chan *models.WebhookEvent
	wg         sync.WaitGroup
	client     *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewDispatcher configures the webhook dispatcher.
// It initializes a channel to queue outgoing events and a standard HTTP client with a 5-second timeout.
func NewDispatcher(es *store.EventStore) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher{
		EventStore: es,
		eventChan:  make(chan *models.WebhookEvent, 100), // Buffer up to 100 events
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start spawns the background worker goroutine to monitor incoming events.
func (d *Dispatcher) Start() {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		for {
			select {
			case event, ok := <-d.eventChan:
				if !ok {
					// Channel was closed, stop the worker
					return
				}
				d.fanOut(event)
			case <-d.ctx.Done():
				// Context cancelled, shut down
				return
			}
		}
	}()
}

// Dispatch pushes an event into the channel queue so the background loop can process it.
// If the queue is completely full, it silently drops the event to avoid blocking handler threads.
func (d *Dispatcher) Dispatch(event *models.WebhookEvent) {
	select {
	case d.eventChan <- event:
	default:
		// Queue full, drop event to prevent application deadlock
	}
}

// Stop cancels the running context and blocks until all active webhooks are finished delivering.
func (d *Dispatcher) Stop() {
	d.cancel()
	close(d.eventChan)
	d.wg.Wait()
}

// fanOut checks all registered subscriptions and spawns a new concurrent delivery worker
// for any subscriber interested in this specific event type (or wildcard '*').
func (d *Dispatcher) fanOut(event *models.WebhookEvent) {
	subs := d.EventStore.ListSubscriptions()
	for _, sub := range subs {
		shouldDeliver := false
		for _, eType := range sub.Events {
			if eType == event.Type || eType == "*" {
				shouldDeliver = true
				break
			}
		}
		if shouldDeliver {
			d.wg.Add(1)
			go func(s *models.WebhookSubscription, e *models.WebhookEvent) {
				defer d.wg.Done()
				d.deliverWithRetry(s, e)
			}(sub, event)
		}
	}
}

// deliverWithRetry sends a POST request with the JSON payload to the subscriber's URL.
// It implements an exponential backoff retry mechanism (max 3 retries) with random jitter to avoid slamming servers.
// It also signs the payload with HMAC-SHA256 using the subscriber's signing secret so they can verify the sender.
func (d *Dispatcher) deliverWithRetry(sub *models.WebhookSubscription, event *models.WebhookEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("Failed to marshal webhook payload", slog.Any("error", err), slog.String("event_id", event.ID))
		return
	}
	maxRetries := 3
	backoff := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Wait before retrying (skip on the very first attempt)
		if attempt > 0 {
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			sleepDuration := backoff + jitter
			slog.Info("Retrying webhook delivery",
				slog.String("url", sub.URL),
				slog.String("event_id", event.ID),
				slog.Int("attempt", attempt),
				slog.Duration("wait", sleepDuration),
			)
			select {
			case <-time.After(sleepDuration):
			case <-d.ctx.Done():
				return // Stop retrying if the dispatcher is shutting down
			}
			backoff *= 2 // Double the wait time for the next attempt
		}

		slog.Info("Attempting webhook delivery",
			slog.String("url", sub.URL),
			slog.String("event_id", event.ID),
			slog.String("event_type", event.Type),
			slog.Int("attempt", attempt),
		)

		req, err := http.NewRequestWithContext(d.ctx, "POST", sub.URL, bytes.NewReader(payload))
		if err != nil {
			slog.Error("Failed to create webhook request", slog.Any("error", err), slog.String("event_id", event.ID))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "AgenticCommerce-Webhook/1.0")

		// Create the HMAC signature so the client can verify this request actually came from our server
		mac := hmac.New(sha256.New, []byte(sub.SigningSecret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Signature", fmt.Sprintf("t=%d, v1=%s", event.CreatedAt.Unix(), signature))

		resp, err := d.client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				slog.Info("Webhook delivered successfully",
					slog.String("url", sub.URL),
					slog.String("event_id", event.ID),
					slog.Int("attempts_needed", attempt+1),
					slog.Int("status_code", resp.StatusCode),
				)
				return // Success! Exit early
			}
			slog.Warn("Webhook delivery failed with status",
				slog.String("url", sub.URL),
				slog.String("event_id", event.ID),
				slog.Int("status_code", resp.StatusCode),
				slog.Int("attempt", attempt),
			)
		} else {
			slog.Warn("Webhook delivery failed with network error",
				slog.String("url", sub.URL),
				slog.String("event_id", event.ID),
				slog.Any("error", err),
				slog.Int("attempt", attempt),
			)
		}
	}

	// We tried 4 times total and still failed. Log it as an error.
	slog.Error("Webhook delivery failed permanently",
		slog.String("url", sub.URL),
		slog.String("event_id", event.ID),
		slog.Int("max_retries", maxRetries),
	)
}

