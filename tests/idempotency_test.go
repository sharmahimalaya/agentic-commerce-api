package tests

import (
	"acommerce_api_endpoint/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIdempotency(t *testing.T) {
	env := SetupTestEnv()
	defer env.Dispatcher.Stop()

	token, _ := env.TokenStore.Create([]models.Scope{models.ScopeCartsWrite}, 0, 2*time.Hour)
	authHeader := "Bearer " + token.Secret
	idemKey := "test-idempotency-key-1"

	var firstCartID string

	tests := []struct {
		name           string
		method         string
		endpoint       string
		authHeader     string
		idemKey        string
		expectedStatus int
		validateResp   func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "First request: Create Cart",
			method:         "POST",
			endpoint:       "/v1/carts",
			authHeader:     authHeader,
			idemKey:        idemKey,
			expectedStatus: http.StatusCreated,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var cart models.Cart
				json.Unmarshal(w.Body.Bytes(), &cart)
				firstCartID = cart.ID
			},
		},
		{
			name:           "Second request (Replay) with same key",
			method:         "POST",
			endpoint:       "/v1/carts",
			authHeader:     authHeader,
			idemKey:        idemKey,
			expectedStatus: http.StatusCreated,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var cart models.Cart
				json.Unmarshal(w.Body.Bytes(), &cart)
				if cart.ID != firstCartID {
					t.Errorf("Expected identical cart ID %s, got %s", firstCartID, cart.ID)
				}
			},
		},
		{
			name:           "Different key should create a new cart",
			method:         "POST",
			endpoint:       "/v1/carts",
			authHeader:     authHeader,
			idemKey:        "test-idempotency-key-2",
			expectedStatus: http.StatusCreated,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var cart models.Cart
				json.Unmarshal(w.Body.Bytes(), &cart)
				if cart.ID == firstCartID {
					t.Error("Expected different cart ID for a different idempotency key")
				}
			},
		},
		{
			name:           "GET requests should not be cached",
			method:         "GET",
			endpoint:       "/v1/products",
			authHeader:     authHeader,
			idemKey:        idemKey,
			expectedStatus: http.StatusForbidden, // Forbidden because token lacks products:read
			validateResp:   nil,
		},
		{
			name:           "Error responses should not be cached (Attempt 1: Error)",
			method:         "POST",
			endpoint:       "/v1/carts",
			authHeader:     "", // Omit auth to trigger 401
			idemKey:        "error-key",
			expectedStatus: http.StatusUnauthorized,
			validateResp:   nil,
		},
		{
			name:           "Error responses should not be cached (Attempt 2: Success)",
			method:         "POST",
			endpoint:       "/v1/carts",
			authHeader:     authHeader, // Now provide auth with the same key
			idemKey:        "error-key",
			expectedStatus: http.StatusCreated,
			validateResp:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.endpoint, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			if tt.idemKey != "" {
				req.Header.Set("Idempotency-Key", tt.idemKey)
			}
			env.Router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.validateResp != nil {
				tt.validateResp(t, w)
			}
		})
	}
}
