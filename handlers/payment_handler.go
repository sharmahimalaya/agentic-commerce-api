package handlers

import (
	"acommerce_api_endpoint/gateway"
	"acommerce_api_endpoint/models"
	"acommerce_api_endpoint/store"
	"acommerce_api_endpoint/webhook"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/gin-gonic/gin"
)

// PaymentHandler deals with the critical checkout process.
// It verifies RS256 JWT purchase mandates signed by the client,
// validates that the cart hasn't been tampered with, checks token spending limits,
// charges the payment gateway, and transitions payment intent states.
type PaymentHandler struct {
	PaymentStore store.PaymentStorer
	CartStore    store.CartStorer
	TokenStore   *store.TokenStore
	Gateway      gateway.Gateway
	Dispatcher   *webhook.Dispatcher
	verifyKey    *rsa.PublicKey // Cached RSA public key decoded during startup
}

// NewPaymentHandler initializes the handler and parses the public key PEM.
// It panics if the key is invalid, because we absolutely need this to verify signatures!
func NewPaymentHandler(ps store.PaymentStorer, cs store.CartStorer, ts *store.TokenStore, pg gateway.Gateway, d *webhook.Dispatcher, publicKeyPEM string) *PaymentHandler {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		panic("Failed to parse public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic("Failed to parse public key: " + err.Error())
	}
	vKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		panic("Public key is not of type RSA public key")
	}

	return &PaymentHandler{
		PaymentStore: ps,
		CartStore:    cs,
		TokenStore:   ts,
		Gateway:      pg,
		Dispatcher:   d,
		verifyKey:    vKey,
	}
}

// CreateIntentInput is what the client sends us to start checking out.
// They must provide the cart ID, the currency they're using, and the JWT mandate.
type CreateIntentInput struct {
	CartID     string `json:"cart_id" binding:"required"`
	Currency   string `json:"currency" binding:"required,oneof=INR USD EUR"`
	MandateJWT string `json:"mandate_jwt" binding:"required"`
}

// MandateClaims matches the structure of the signed JWT claims from the client.
// It includes a hash of the cart contents and the authorized checkout amount.
type MandateClaims struct {
	MandateID string `json:"mandate_id"`
	CartHash  string `json:"cart_hash"`
	AmountPa  int64  `json:"amount_pa"`
	jwt.RegisteredClaims
}

// CreateIntent handles POST /v1/payment-intents.
// It reads the cart, verifies the JWT signature (RS256) and claims,
// checks that the cart contents/total match what was signed (to prevent price tampering),
// creates a payment intent in the store, and transitions it to 'requires_confirmation'.
func (h *PaymentHandler) CreateIntent(c *gin.Context) {
	var input CreateIntentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch the cart from memory
	cart, err := h.CartStore.Get(input.CartID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cart not found"})
		return
	}
	if len(cart.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot check out an empty cart"})
		return
	}

	// Verify JWT signature using our public key
	var mc MandateClaims
	parsedToken, err := jwt.ParseWithClaims(input.MandateJWT, &mc, func(t *jwt.Token) (interface{}, error) {
		// Make sure they are using RSA (RS256) to sign this and not some weak fallback
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.verifyKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer("commerce_api"))
	if err != nil || !parsedToken.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mandate token: " + err.Error()})
		return
	}

	// Recreate the cart hash locally and compare it with what the client signed.
	// This makes sure the client didn't secretly change product quantities/prices after signing!
	var parts []string
	for _, it := range cart.Items {
		parts = append(parts, fmt.Sprintf("%s:%d", it.ProductID, it.Quantity))
	}
	itemsStr := strings.Join(parts, ",")
	// Format is: cartID|productId:quantity,productId:quantity|totalPaise
	hashInput := fmt.Sprintf("%s|%s|%d", cart.ID, itemsStr, cart.TotalPaise)
	sum := sha256.Sum256([]byte(hashInput))
	localHash := hex.EncodeToString(sum[:])

	// Strict checks to avoid fraud
	if mc.CartHash != localHash {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mandate cart hash mismatch"})
		return
	}
	if mc.AmountPa != cart.TotalPaise {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mandate amount mismatch"})
		return
	}

	// Save the payment intent in Created status
	intent := h.PaymentStore.Create(cart.ID, cart.TotalPaise, input.Currency, mc.MandateID, input.MandateJWT)

	// Transition status to RequiresConfirmation so it's ready to be confirmed (charged)
	if err := h.PaymentStore.TransitionStatus(intent.ID, models.PaymentStatusRequiresConfirmation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize payment flow"})
		return
	}
	var er error
	intent, er = h.PaymentStore.Get(intent.ID)
	if er != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, intent)

	// Fire an event to notify subscribers (e.g. shipping, metrics)
	evt := h.Dispatcher.EventStore.RecordEvent("payment_intent.created", intent)
	h.Dispatcher.Dispatch(evt)
}

// GetIntent handles GET /v1/payment-intents/:id.
// Retrieves a payment intent by its unique ID.
func (h *PaymentHandler) GetIntent(c *gin.Context) {
	id := c.Param("id")
	intent, err := h.PaymentStore.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, intent)
}

// ConfirmIntent handles POST /v1/payment-intents/:id/confirm.
// It verifies the authorization token's spend limit, calls the payment gateway,
// updates the status to succeeded or failed, and fires webhook events.
func (h *PaymentHandler) ConfirmIntent(c *gin.Context) {
	id := c.Param("id")

	intent, err := h.PaymentStore.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Spend-limit check: make sure the bearer token hasn't exceeded its lifetime budget limit.
	tokenVal, exists := c.Get("Token")
	if exists {
		token := tokenVal.(*models.AuthToken)
		if err := h.TokenStore.RecordSpend(token.Secret, intent.AmountPaise); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "spend check failed: " + err.Error()})
			return
		}
	}

	// Step 1: Charge the gateway FIRST.
	// We want to be sure the bank transaction cleared before we mark it succeeded.
	err = h.Gateway.Charge(intent.AmountPaise, intent.Currency)
	if err != nil {
		// Gateway rejected the charge. Mark status as failed and emit failure webhook event.
		_ = h.PaymentStore.TransitionStatus(id, models.PaymentStatusFailed)
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "payment gateway charge failed: " + err.Error()})

		if intentFailed, fetchErr := h.PaymentStore.Get(id); fetchErr == nil {
			evt := h.Dispatcher.EventStore.RecordEvent("payment_intent.failed", intentFailed)
			h.Dispatcher.Dispatch(evt)
		}
		return
	}

	// Step 2: Charge succeeded — transition state machine to Succeeded status.
	err = h.PaymentStore.TransitionStatus(id, models.PaymentStatusSucceeded)
	if err != nil {
		var transitionErr store.ErrInvalidTransition
		if errors.As(err, &transitionErr) {
			if transitionErr.Current == models.PaymentStatusSucceeded {
				c.JSON(http.StatusConflict, gin.H{"error": "payment has already succeeded"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Step 3: Fetch updated status and respond with final status representation.
	intent, err = h.PaymentStore.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, intent)

	// Step 4: Emit success event asynchronously so warehouse or notification services can pick it up.
	evt := h.Dispatcher.EventStore.RecordEvent("payment_intent.succeeded", intent)
	h.Dispatcher.Dispatch(evt)
}

