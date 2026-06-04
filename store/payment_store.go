package store

import (
	"agentic-commerce/models"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrPaymentNotFound = errors.New("payment intent not found")

type ErrInvalidTransition struct {
	Current models.PaymentStatus
	Target  models.PaymentStatus
}

func (e ErrInvalidTransition) Error() string {
	return fmt.Sprintf("cannot transition payment intent from %s to %s", e.Current, e.Target)
}

type PaymentStore struct {
	mu      sync.RWMutex
	intents map[string]*models.PaymentIntent
}

func NewPaymentStore() *PaymentStore {
	return &PaymentStore{
		intents: make(map[string]*models.PaymentIntent),
	}
}

func (s *PaymentStore) Create(cartID string, amount int64, currency string) *models.PaymentIntent {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent := &models.PaymentIntent{
		ID:          "pi_" + uuid.New().String(),
		CartID:      cartID,
		AmountPaise: amount,
		Currency:    currency,
		Status:      models.PaymentStatusCreated,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	s.intents[intent.ID] = intent
	return intent
}

func (s *PaymentStore) Get(id string) (*models.PaymentIntent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	intent, exists := s.intents[id]
	if !exists {
		return nil, ErrPaymentNotFound
	}
	return intent, nil
}

func (s *PaymentStore) TransitionStatus(id string, to models.PaymentStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, exists := s.intents[id]
	if !exists {
		return ErrPaymentNotFound
	}

	allowed := false
	switch intent.Status {
	case models.PaymentStatusCreated:
		allowed = (to == models.PaymentStatusRequiresConfirmation)
	case models.PaymentStatusRequiresConfirmation:
		allowed = (to == models.PaymentStatusSucceeded || to == models.PaymentStatusFailed)
	}

	if !allowed {
		return ErrInvalidTransition{Current: intent.Status, Target: to}
	}

	intent.Status = to
	intent.UpdatedAt = time.Now()
	return nil
}
