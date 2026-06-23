package store

import (
	"acommerce_api_endpoint/models"

	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrCartNotFound is what we throw when someone asks for a cart ID that doesn't exist.
var ErrCartNotFound = errors.New("cart not found")

// CartStorer is an interface so we can mock this store easily in tests.
// Basically, anything that can Create, Get, and Save a cart can be used.
type CartStorer interface {
	Create() *models.Cart
	Get(id string) (*models.Cart, error)
	Save(cart *models.Cart)
}

// CartStore is our simple in-memory database for shopping carts.
// It uses a map under the hood and a Mutex to prevent weird concurrency issues.
type CartStore struct {
	mu    sync.RWMutex
	carts map[string]*models.Cart
}

// NewCartStore initializes our store with an empty map.
func NewCartStore() *CartStore {
	return &CartStore{
		carts: make(map[string]*models.Cart),
	}
}

// Create makes a brand new empty cart, assigns a UUID, and saves it in the map.
// We return a copy of the cart so the caller can't accidentally mutate the store's copy directly.
func (s *CartStore) Create() *models.Cart {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart := &models.Cart{
		ID:        uuid.New().String(),
		Items:     []models.CartItem{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.carts[cart.ID] = cart

	// Deep copy time! Return a copy so the handler doesn't modify the map pointer directly.
	copied := *cart
	return &copied
}

// Get finds a cart by its ID. We do a deep copy of the items slice too,
// because otherwise Go maps might share underlying array pointers, which leads to race conditions.
func (s *CartStore) Get(id string) (*models.Cart, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cart, exists := s.carts[id]
	if !exists {
		return nil, ErrCartNotFound
	}

	// We make a copy of the cart struct itself
	copiedCart := *cart
	// And we must copy the slice of items! If we don't, two goroutines could modify the same slice.
	copiedItems := make([]models.CartItem, len(cart.Items))
	copy(copiedItems, cart.Items)
	copiedCart.Items = copiedItems

	return &copiedCart, nil
}

// Save updates the cart in our store and recalculates the total price in Paise (cents).
// We copy the cart before saving it to make sure the store is the single source of truth.
func (s *CartStore) Save(cart *models.Cart) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart.UpdatedAt = time.Now()
	// Loop through items to calculate the total cost in Paise
	var total int64
	for _, item := range cart.Items {
		total += item.PricePaise * int64(item.Quantity)
	}
	cart.TotalPaise = total

	// Deep copy again so the store has its own untouched copy of the cart data
	copiedCart := *cart
	copiedItems := make([]models.CartItem, len(cart.Items))
	copy(copiedItems, cart.Items)
	copiedCart.Items = copiedItems
	s.carts[cart.ID] = &copiedCart
}

