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
	}
}

func (d *Dispatcher) Stop() {
	d.cancel()
	close(d.eventChan)
	d.wg.Wait()
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
		return
	}
	maxRetries := 3
	backoff := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			sleepDuration := backoff + jitter
			select {
			case <-time.After(sleepDuration):
			case <-d.ctx.Done():
				return
			}
			backoff *= 2
		}
		req, err := http.NewRequestWithContext(d.ctx, "POST", sub.URL, bytes.NewReader(payload))
		if err != nil {
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
				return
			}
		}
	}
}
