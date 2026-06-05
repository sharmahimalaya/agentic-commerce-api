package store

import (
	"acommerce_api_endpoint/models"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var ErrInvalidToken = errors.New("invalid or expired token")

var ErrSpendLimitExceeded = errors.New("spend limit exceeded for this token")

type TokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*models.AuthToken
}

func NewTokenStore() *TokenStore {
	return &TokenStore{
		tokens: make(map[string]*models.AuthToken),
	}
}

func (s *TokenStore) Create(scopes []models.Scope, spendLimit int64, duration time.Duration) (*models.AuthToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}

	secret := "ac_tok_" + hex.EncodeToString(bytes)

	token := &models.AuthToken{
		ID:                hex.EncodeToString(bytes[:4]),
		Secret:            secret,
		Scopes:            scopes,
		SpendLimitPaise:   spendLimit,
		CurrentSpendPaise: 0,
		ExpiresAt:         time.Now().Add(duration),
		CreatedAt:         time.Now(),
	}

	s.tokens[secret] = token
	return token, nil
}

func (s *TokenStore) Get(secret string) (*models.AuthToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, exists := s.tokens[secret]
	if !exists {
		return nil, ErrInvalidToken
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, ErrInvalidToken
	}
	return token, nil
}

func (s *TokenStore) RecordSpend(secret string, amount int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, exists := s.tokens[secret]

	if !exists {
		return ErrInvalidToken
	}

	if time.Now().After(token.ExpiresAt) {
		return ErrInvalidToken
	}

	if token.SpendLimitPaise > 0 && token.CurrentSpendPaise+amount > token.SpendLimitPaise {
		return ErrSpendLimitExceeded
	}

	token.CurrentSpendPaise += amount
	return nil
}
