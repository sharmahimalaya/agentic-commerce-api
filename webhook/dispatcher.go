package webhook

import (
	"agentic-commerce/models"
	"agentic-commerce/store"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type Dispatcher struct {
	EventStore *store.EventStore
	eventChan  chan *models.WebhookEvent
	wg         sync.WaitGroup
	client     *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewDispatcher(es *store.EventStore) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher{
		EventStore: es,
		eventChan:  make(chan *models.WebhookEvent, 100),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (d *Dispatcher) Start() {
	log.Println("[WEBHOOK] Starting dispatcher worker loop...")
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		for {
			select {
			case event, ok := <-d.eventChan:
				if !ok {
					return
				}
				d.fanOut(event)
			case <-d.ctx.Done():
				return
			}
		}
	}()
}

func (d *Dispatcher) Dispatch(event *models.WebhookEvent) {
	select {
	case d.eventChan <- event:
	default:
		log.Printf("[WEBHOOK] Event channel full, dropping event: %s", event.ID)
	}
}

func (d *Dispatcher) Stop() {
	log.Println("[WEBHOOK] Stopping dispatcher, flushing events...")
	d.cancel()
	close(d.eventChan)
	d.wg.Wait()
	log.Println("[WEBHOOK] Dispatcher stopped gracefully.")
}

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

func (d *Dispatcher) deliverWithRetry(sub *models.WebhookSubscription, event *models.WebhookEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("[WEBHOOK] Failed to marshal event %s: %v", event.ID, err)
		return
	}
	maxRetries := 3
	backoff := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			sleepDuration := backoff + jitter
			log.Printf("[WEBHOOK] Retrying delivery for event %s to %s in %v (Attempt %d/%d)", event.ID, sub.URL, sleepDuration, attempt, maxRetries)
			select {
			case <-time.After(sleepDuration):
			case <-d.ctx.Done():
				return
			}
			backoff *= 2
		}
		req, err := http.NewRequestWithContext(d.ctx, "POST", sub.URL, bytes.NewReader(payload))
		if err != nil {
			log.Printf("[WEBHOOK] Failed to create request for event %s: %v", event.ID, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "AgenticCommerce-Webhook/1.0")

		mac := hmac.New(sha256.New, []byte(sub.SigningSecret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Signature", fmt.Sprintf("t=%d, v1=%s", event.CreatedAt.Unix(), signature))
		resp, err := d.client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				log.Printf("[WEBHOOK] Successfully delivered event %s to %s (Status: %d)", event.ID, sub.URL, resp.StatusCode)
				return
			}
			log.Printf("[WEBHOOK] Bad response code delivering event %s to %s: %d", event.ID, sub.URL, resp.StatusCode)
		} else {
			log.Printf("[WEBHOOK] Delivery failed for event %s to %s: %v", event.ID, sub.URL, err)
		}
	}
	log.Printf("[WEBHOOK] Max retries reached. Delivery failed for event %s to %s", event.ID, sub.URL)
}
