package tests

import (
	"acommerce_api_endpoint/models"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthAndTokenFlow(t *testing.T) {
	env := SetupTestEnv()
	defer env.Dispatcher.Stop()

	reqBody := map[string]interface{}{
		"scopes":         []string{"products:read"},
		"duration_hours": 2,
	}

	var validTokenSecret string

	t.Run("Create Token Scenarios", func(t *testing.T) {
		tests := []struct {
			name           string
			adminKey       string
			expectedStatus int
			extractToken   bool
		}{
			{"Without Admin Key", "", http.StatusUnauthorized, false},
			{"With Invalid Admin Key", "wrong_admin_key", http.StatusUnauthorized, false},
			{"With Correct Admin Key", "test_admin_key", http.StatusCreated, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				bodyBytes, _ := json.Marshal(reqBody)
				req, _ := http.NewRequest("POST", "/v1/tokens", bytes.NewBuffer(bodyBytes))
				if tt.adminKey != "" {
					req.Header.Set("X-Admin-Key", tt.adminKey)
				}
				env.Router.ServeHTTP(w, req)

				if w.Code != tt.expectedStatus {
					t.Errorf("Expected %d, got %d", tt.expectedStatus, w.Code)
				}

				if tt.extractToken && w.Code == http.StatusCreated {
					var tokenResponse models.AuthToken
					if err := json.Unmarshal(w.Body.Bytes(), &tokenResponse); err != nil {
						t.Fatalf("Failed to parse token response: %v", err)
					}
					if tokenResponse.Secret == "" {
						t.Error("Generated token secret is empty")
					}
					validTokenSecret = tokenResponse.Secret
				}
			})
		}
	})

	t.Run("Access Protected Endpoint Scenarios", func(t *testing.T) {
		tests := []struct {
			name           string
			method         string
			endpoint       string
			authHeader     string
			expectedStatus int
		}{
			{"Missing Authorization Header", "GET", "/v1/products", "", http.StatusUnauthorized},
			{"Malformed Header", "GET", "/v1/products", "Bearer", http.StatusUnauthorized},
			{"Basic Auth Header", "GET", "/v1/products", "Basic " + validTokenSecret, http.StatusUnauthorized},
			{"Incorrect Scope", "POST", "/v1/carts", "Bearer " + validTokenSecret, http.StatusForbidden},
			{"Valid Scope", "GET", "/v1/products", "Bearer " + validTokenSecret, http.StatusOK},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				var req *http.Request
				if tt.method == "POST" {
					req, _ = http.NewRequest(tt.method, tt.endpoint, bytes.NewBuffer([]byte("{}")))
				} else {
					req, _ = http.NewRequest(tt.method, tt.endpoint, nil)
				}

				if tt.authHeader != "" {
					req.Header.Set("Authorization", tt.authHeader)
				}
				env.Router.ServeHTTP(w, req)

				if w.Code != tt.expectedStatus {
					t.Errorf("Expected %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
				}
			})
		}
	})

	t.Run("Access With Expired Token", func(t *testing.T) {
		expiredTok, err := env.TokenStore.Create([]models.Scope{models.ScopeProductsRead}, 0, -1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to create expired token: %v", err)
		}
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/products", nil)
		req.Header.Set("Authorization", "Bearer "+expiredTok.Secret)
		env.Router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized for expired token, got %d", w.Code)
		}
	})
}
