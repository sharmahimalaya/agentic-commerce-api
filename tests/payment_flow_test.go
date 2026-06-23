package tests

import (
	"acommerce_api_endpoint/models"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPaymentFlow(t *testing.T) {
	env := SetupTestEnv()
	defer env.Dispatcher.Stop()

	// 1. Setup Data
	token, _ := env.TokenStore.Create([]models.Scope{
		models.ScopeCartsWrite, models.ScopeCartsRead,
		models.ScopePaymentsWrite, models.ScopePaymentsRead,
	}, 15000, 2*time.Hour)
	authHeader := "Bearer " + token.Secret

	cart := env.CartStore.Create()
	prod, _ := env.ProductStore.GetById("prod_1") // Rs. 10.00 (1000 paise)
	cart.Items = append(cart.Items, models.CartItem{
		ProductID:  prod.ID,
		Quantity:   5, // total: 5000 paise
		PricePaise: prod.PricePaise,
	})
	env.CartStore.Save(cart)

	hashInput := fmt.Sprintf("%s|prod_1:5|5000", cart.ID)
	sum := sha256.Sum256([]byte(hashInput))
	correctHash := hex.EncodeToString(sum[:])

	badTokenAlg, _ := GenerateMandateJWT("mandate_123", correctHash, 5000, "HS256")
	badTokenHash, _ := GenerateMandateJWT("mandate_123", "wronghashhere", 5000, "RS256")
	validToken, _ := GenerateMandateJWT("mandate_123", correctHash, 5000, "RS256")

	var intentID string

	// Table for creating and confirming the first intent
	t.Run("Intent Lifecycle", func(t *testing.T) {
		tests := []struct {
			name           string
			method         string
			endpointFunc   func() string
			body           interface{}
			expectedStatus int
			validateResp   func(t *testing.T, w *httptest.ResponseRecorder)
		}{
			{
				name:           "Create Intent - Weak Algorithm (HS256)",
				method:         "POST",
				endpointFunc:   func() string { return "/v1/payment-intents" },
				body:           map[string]interface{}{"cart_id": cart.ID, "currency": "INR", "mandate_jwt": badTokenAlg},
				expectedStatus: http.StatusBadRequest,
			},
			{
				name:           "Create Intent - Mismatched Cart Hash",
				method:         "POST",
				endpointFunc:   func() string { return "/v1/payment-intents" },
				body:           map[string]interface{}{"cart_id": cart.ID, "currency": "INR", "mandate_jwt": badTokenHash},
				expectedStatus: http.StatusBadRequest,
			},
			{
				name:           "Create Intent - Happy Path",
				method:         "POST",
				endpointFunc:   func() string { return "/v1/payment-intents" },
				body:           map[string]interface{}{"cart_id": cart.ID, "currency": "INR", "mandate_jwt": validToken},
				expectedStatus: http.StatusCreated,
				validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
					var intent models.PaymentIntent
					json.Unmarshal(w.Body.Bytes(), &intent)
					if intent.Status != models.PaymentStatusRequiresConfirmation {
						t.Errorf("Expected status 'requires_confirmation', got %s", intent.Status)
					}
					intentID = intent.ID
				},
			},
			{
				name:           "Confirm Intent - Gateway Success",
				method:         "POST",
				endpointFunc:   func() string { return fmt.Sprintf("/v1/payment-intents/%s/confirm", intentID) },
				body:           nil,
				expectedStatus: http.StatusOK,
				validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
					var confirmedIntent models.PaymentIntent
					json.Unmarshal(w.Body.Bytes(), &confirmedIntent)
					if confirmedIntent.Status != models.PaymentStatusSucceeded {
						t.Errorf("Expected final status 'succeeded', got %s", confirmedIntent.Status)
					}

					// Verify spend limit decreased correctly
					tokFromStore, _ := env.TokenStore.Get(token.Secret)
					if tokFromStore.CurrentSpendPaise != 5000 {
						t.Errorf("Expected CurrentSpendPaise to be 5000, got %d", tokFromStore.CurrentSpendPaise)
					}
				},
			},
			{
				name:           "Double Confirmation Replay Check",
				method:         "POST",
				endpointFunc:   func() string { return fmt.Sprintf("/v1/payment-intents/%s/confirm", intentID) },
				body:           nil,
				expectedStatus: http.StatusConflict,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				var reqBody []byte
				if tt.body != nil {
					reqBody, _ = json.Marshal(tt.body)
				}
				req, _ := http.NewRequest(tt.method, tt.endpointFunc(), bytes.NewBuffer(reqBody))
				req.Header.Set("Authorization", authHeader)
				env.Router.ServeHTTP(w, req)

				if w.Code != tt.expectedStatus {
					t.Fatalf("Expected %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
				}
				if tt.validateResp != nil {
					tt.validateResp(t, w)
				}
			})
		}
	})

	t.Run("Spend Limit Exceeded", func(t *testing.T) {
		cart2 := env.CartStore.Create()
		cart2.Items = append(cart2.Items, models.CartItem{ProductID: prod.ID, Quantity: 11, PricePaise: prod.PricePaise}) // 11,000 paise (exceeds remaining 10,000)
		env.CartStore.Save(cart2)

		hashInput2 := fmt.Sprintf("%s|prod_1:11|11000", cart2.ID)
		sum2 := sha256.Sum256([]byte(hashInput2))
		validToken2, _ := GenerateMandateJWT("mandate_456", hex.EncodeToString(sum2[:]), 11000, "RS256")

		w := httptest.NewRecorder()
		intentBody2, _ := json.Marshal(map[string]interface{}{"cart_id": cart2.ID, "currency": "INR", "mandate_jwt": validToken2})
		req, _ := http.NewRequest("POST", "/v1/payment-intents", bytes.NewBuffer(intentBody2))
		req.Header.Set("Authorization", authHeader)
		env.Router.ServeHTTP(w, req)

		var intent2 models.PaymentIntent
		json.Unmarshal(w.Body.Bytes(), &intent2)

		// Confirm to trigger spend limit check
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("POST", fmt.Sprintf("/v1/payment-intents/%s/confirm", intent2.ID), nil)
		req2.Header.Set("Authorization", authHeader)
		env.Router.ServeHTTP(w2, req2)

		if w2.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden for spend limit exceeded, got %d", w2.Code)
		}
	})

	t.Run("Gateway Failure Simulation", func(t *testing.T) {
		token3, _ := env.TokenStore.Create([]models.Scope{models.ScopeCartsWrite, models.ScopePaymentsWrite}, 0, 2*time.Hour)
		authHeader3 := "Bearer " + token3.Secret

		cart3 := env.CartStore.Create()
		cart3.Items = append(cart3.Items, models.CartItem{ProductID: prod.ID, Quantity: 1, PricePaise: prod.PricePaise})
		env.CartStore.Save(cart3)

		hashInput3 := fmt.Sprintf("%s|prod_1:1|1000", cart3.ID)
		sum3 := sha256.Sum256([]byte(hashInput3))
		validToken3, _ := GenerateMandateJWT("mandate_789", hex.EncodeToString(sum3[:]), 1000, "RS256")

		w := httptest.NewRecorder()
		intentBody3, _ := json.Marshal(map[string]interface{}{"cart_id": cart3.ID, "currency": "INR", "mandate_jwt": validToken3})
		req, _ := http.NewRequest("POST", "/v1/payment-intents", bytes.NewBuffer(intentBody3))
		req.Header.Set("Authorization", authHeader3)
		env.Router.ServeHTTP(w, req)

		var intent3 models.PaymentIntent
		json.Unmarshal(w.Body.Bytes(), &intent3)

		// Instruct mock gateway to fail
		env.Gateway.ShouldFail = true
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("POST", fmt.Sprintf("/v1/payment-intents/%s/confirm", intent3.ID), nil)
		req2.Header.Set("Authorization", authHeader3)
		env.Router.ServeHTTP(w2, req2)

		if w2.Code != http.StatusPaymentRequired {
			t.Errorf("Expected 402 Payment Required for failed gateway charge, got %d", w2.Code)
		}

		intent3FromStore, _ := env.PaymentStore.Get(intent3.ID)
		if intent3FromStore.Status != models.PaymentStatusFailed {
			t.Errorf("Expected payment status failed, got %s", intent3FromStore.Status)
		}
	})
}
