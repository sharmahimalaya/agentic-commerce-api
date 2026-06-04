package store

import (
	"agentic-commerce/models"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type EventStore struct {
	subMu sync.RWMutex
	subs  map[string]*models.WebhookSubscription

	eventsMu sync.RWMutex
	events   []*models.WebhookEvent
}

func NewEventStore() *EventStore {
	return &EventStore{
		subs:   make(map[string]*models.WebhookSubscription),
		events: make([]*models.WebhookEvent, 0),
	}
}

func (s *EventStore) CreateSubscription(url string, events []string) (*models.WebhookSubscription, error) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}

	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	secret := "whsec_" + hex.EncodeToString(secretBytes)
	sub := &models.WebhookSubscription{
		ID:            "sub_" + hex.EncodeToString(idBytes),
		URL:           url,
		Events:        events,
		SigningSecret: secret,
		CreatedAt:     time.Now(),
	}

	s.subs[sub.ID] = sub
	return sub, nil
}

func (s *EventStore) ListSubscriptions() []*models.WebhookSubscription {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	list := make([]*models.WebhookSubscription, 0, len(s.subs))
	for _, sub := range s.subs {
		list = append(list, sub)
	}
	return list
}

func (s *EventStore) DeleteSubscription(id string) error {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	if _, exists := s.subs[id]; !exists {
		return errors.New("subscription not found")
	}
	delete(s.subs, id)
	return nil
}

func (s *EventStore) RecordEvent(eventType string, data interface{}) *models.WebhookEvent {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()

	event := &models.WebhookEvent{
		ID:        "evt_" + uuid.NewString(),
		Type:      eventType,
		Data:      data,
		CreatedAt: time.Now(),
	}
	s.events = append(s.events, event)
	return event

}
