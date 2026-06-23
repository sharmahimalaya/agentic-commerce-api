package tests

import (
	"acommerce_api_endpoint/models"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCartFlow(t *testing.T) {
	env := SetupTestEnv()
	defer env.Dispatcher.Stop()

	token, _ := env.TokenStore.Create([]models.Scope{models.ScopeCartsWrite, models.ScopeCartsRead}, 0, 2*time.Hour)
	authHeader := "Bearer " + token.Secret

	var cartID string

	tests := []struct {
		name           string
		method         string
		endpointFunc   func() string
		body           interface{}
		expectedStatus int
		validateResp   func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "1. Create Cart",
			method:         "POST",
			endpointFunc:   func() string { return "/v1/carts" },
			body:           nil,
			expectedStatus: http.StatusCreated,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var cart models.Cart
				json.Unmarshal(w.Body.Bytes(), &cart)
				if cart.ID == "" {
					t.Error("Cart ID was not generated")
				}
				cartID = cart.ID // Save state for next tests
			},
		},
		{
			name:           "2. Add Item (Happy Path)",
			method:         "POST",
			endpointFunc:   func() string { return fmt.Sprintf("/v1/carts/%s/items", cartID) },
			body:           map[string]interface{}{"product_id": "prod_1", "quantity": 2},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var updatedCart models.Cart
				json.Unmarshal(w.Body.Bytes(), &updatedCart)
				if len(updatedCart.Items) != 1 || updatedCart.Items[0].ProductID != "prod_1" {
					t.Errorf("Expected 1 item (prod_1), got items: %+v", updatedCart.Items)
				}
				if updatedCart.Items[0].Quantity != 2 {
					t.Errorf("Expected quantity 2, got %d", updatedCart.Items[0].Quantity)
				}
				if updatedCart.TotalPaise != 2000 {
					t.Errorf("Expected total 2000 paise, got %d", updatedCart.TotalPaise)
				}
			},
		},
		{
			name:           "3. Add Item with Insufficient Stock",
			method:         "POST",
			endpointFunc:   func() string { return fmt.Sprintf("/v1/carts/%s/items", cartID) },
			body:           map[string]interface{}{"product_id": "prod_2", "quantity": 51},
			expectedStatus: http.StatusBadRequest,
			validateResp:   nil,
		},
		{
			name:           "4. Update Item Quantity",
			method:         "PUT",
			endpointFunc:   func() string { return fmt.Sprintf("/v1/carts/%s/items/prod_1", cartID) },
			body:           map[string]interface{}{"quantity": 5},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var cartAfterUpdate models.Cart
				json.Unmarshal(w.Body.Bytes(), &cartAfterUpdate)
				if cartAfterUpdate.Items[0].Quantity != 5 {
					t.Errorf("Expected quantity 5 after update, got %d", cartAfterUpdate.Items[0].Quantity)
				}
				if cartAfterUpdate.TotalPaise != 5000 {
					t.Errorf("Expected total 5000 paise, got %d", cartAfterUpdate.TotalPaise)
				}
			},
		},
		{
			name:           "5. Remove Item",
			method:         "DELETE",
			endpointFunc:   func() string { return fmt.Sprintf("/v1/carts/%s/items/prod_1", cartID) },
			body:           nil,
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var cartAfterDelete models.Cart
				json.Unmarshal(w.Body.Bytes(), &cartAfterDelete)
				if len(cartAfterDelete.Items) != 0 {
					t.Errorf("Expected empty cart, got %d items", len(cartAfterDelete.Items))
				}
			},
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
}

func TestConcurrentStockChecking(t *testing.T) {
	env := SetupTestEnv()
	defer env.Dispatcher.Stop()

	token, _ := env.TokenStore.Create([]models.Scope{models.ScopeCartsWrite, models.ScopeCartsRead}, 0, 2*time.Hour)
	authHeader := "Bearer " + token.Secret
	cart := env.CartStore.Create()

	t.Run("Exceed Stock", func(t *testing.T) {
		w := httptest.NewRecorder()
		itemBody, _ := json.Marshal(map[string]interface{}{
			"product_id": "prod_2",
			"quantity":   51, // Stock is 50
		})
		req, _ := http.NewRequest("POST", fmt.Sprintf("/v1/carts/%s/items", cart.ID), bytes.NewBuffer(itemBody))
		req.Header.Set("Authorization", authHeader)
		env.Router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request when exceeding stock 50, got %d", w.Code)
		}
	})
}
