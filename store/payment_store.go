package store

import (
	"acommerce_api_endpoint/models"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrPaymentNotFound is returned when the requested payment ID doesn't exist in our memory map.
var ErrPaymentNotFound = errors.New("payment intent not found")

// ErrInvalidTransition is a custom error indicating the payment is trying to go to an illegal state.
// We store both the current state and target state so we can print a friendly, debuggable error message.
type ErrInvalidTransition struct {
	Current models.PaymentStatus
	Target  models.PaymentStatus
}

// Error formats the transition failure so it reads nicely in logs.
func (e ErrInvalidTransition) Error() string {
	return fmt.Sprintf("cannot transition payment intent from %s to %s", e.Current, e.Target)
}

// PaymentStore manages our in-memory database of payment intents.
// Like the cart store, we need a RWMutex because multiple HTTP requests could check or update payment status concurrently.
type PaymentStore struct {
	mu      sync.RWMutex
	intents map[string]*models.PaymentIntent
}

// PaymentStorer is the interface for PaymentStore, making unit testing and mocking a breeze.
type PaymentStorer interface {
	Create(cartID string, amount int64, currency string, mandateID string, mandateJWT string) *models.PaymentIntent
	Get(id string) (*models.PaymentIntent, error)
	TransitionStatus(id string, to models.PaymentStatus) error
}

// NewPaymentStore creates an empty payment database map.
func NewPaymentStore() *PaymentStore {
	return &PaymentStore{
		intents: make(map[string]*models.PaymentIntent),
	}
}

// Create generates a new payment intent starting in the 'created' status.
// We return a copy of the intent to protect store internals from concurrent edits.
func (s *PaymentStore) Create(cartID string, amount int64, currency string, mandateID string, mandateJWT string) *models.PaymentIntent {
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
		MandateID:   mandateID,
		MandateJWT:  mandateJWT,
	}

	s.intents[intent.ID] = intent

	// Deep copy to prevent race conditions from shared pointers.
	copied := *intent
	return &copied
}

// Get finds a payment intent by ID. We return a deep copy.
func (s *PaymentStore) Get(id string) (*models.PaymentIntent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	intent, exists := s.intents[id]
	if !exists {
		return nil, ErrPaymentNotFound
	}

	copied := *intent
	return &copied, nil
}

// TransitionStatus moves the payment along its state machine.
// We strictly enforce the allowed state transitions here (Created -> RequiresConfirmation -> Succeeded or Failed).
// If a transition is illegal (e.g. going back to Created from Succeeded), we block it and return ErrInvalidTransition.
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
		// We can only go from Created to RequiresConfirmation
		allowed = (to == models.PaymentStatusRequiresConfirmation)
	case models.PaymentStatusRequiresConfirmation:
		// From RequiresConfirmation we can either succeed or fail
		allowed = (to == models.PaymentStatusSucceeded || to == models.PaymentStatusFailed)
	}

	if !allowed {
		return ErrInvalidTransition{Current: intent.Status, Target: to}
	}

	intent.Status = to
	intent.UpdatedAt = time.Now()
	return nil
}

