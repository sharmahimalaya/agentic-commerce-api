package store

import (
	"acommerce_api_endpoint/models"

	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrCartNotFound = errors.New("cart not found")

type CartStore struct {
	mu    sync.RWMutex
	carts map[string]*models.Cart
}

func NewCartStore() *CartStore {
	return &CartStore{
		carts: make(map[string]*models.Cart),
	}
}

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
	return cart
}

func (s *CartStore) Get(id string) (*models.Cart, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cart, exists := s.carts[id]
	if !exists {
		return nil, ErrCartNotFound
	}

	return cart, nil
}

func (s *CartStore) Save(cart *models.Cart) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart.UpdatedAt = time.Now()
	var total int64
	for _, item := range cart.Items {
		total += item.PricePaise * int64(item.Quantity)
	}
	cart.TotalPaise = total
	s.carts[cart.ID] = cart
}
