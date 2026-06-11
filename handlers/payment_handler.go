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

type PaymentHandler struct {
	PaymentStore *store.PaymentStore
	CartStore    *store.CartStore
	TokenStore   *store.TokenStore
	Gateway      gateway.Gateway
	Dispatcher   *webhook.Dispatcher
}

func NewPaymentHandler(ps *store.PaymentStore, cs *store.CartStore, ts *store.TokenStore, pg gateway.Gateway, d *webhook.Dispatcher) *PaymentHandler {
	return &PaymentHandler{
		PaymentStore: ps,
		CartStore:    cs,
		TokenStore:   ts,
		Gateway:      pg,
		Dispatcher:   d,
	}
}

var (
	publicKeyPEM = []byte(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3EHJuicZBmMBXcZuEGGq
ODBO/C52qAnFCftKWPVA3oTSG5i7sHSfzzn6SEWnWZQYxyJgX7UMdl54hv7J2SWO
IfwRtYipjSZwPlNJMFIqL5/qz6KMXqFNxaS4x45UffECOSdm65afV8JNJXKxMbvi
UCjLMNFV2xr8sJIdGEizNmW85s4Hw6VsI9Lql27hox9IUL54SkqKOcR0AjtfG27P
Ku/Vtr7C8zpVf88468csGx7l9wiJDZYbr/keL1bk9EQimljIGm7sD7WW1vjGf8pg
JjMY927D4sN29GkleD7onGfkrji4+NG3r/S5ZvRes0V5mCtKAsUO5rRnt/Ras98P
lwIDAQAB
-----END PUBLIC KEY-----`)
	verifyKey *rsa.PublicKey
)

func init() {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		panic("Failed to parse public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	var ok bool
	verifyKey, ok = pub.(*rsa.PublicKey)
	if !ok {
		panic("Not correct format for public key")
	}
}

type CreateIntentInput struct {
	CartID     string `json:"cart_id" binding:"required"`
	Currency   string `json:"currency" binding:"required,oneof=INR USD EUR"`
	MandateJWT string `json:"mandate_jwt" binding:"required"`
}

type MandateClaims struct {
	MandateID string `json:"mandate_id"`
	CartHash  string `json:"cart_hash"`
	AmountPa  int64  `json:"amount_pa"`
	jwt.RegisteredClaims
}

func (h *PaymentHandler) CreateIntent(c *gin.Context) {
	var input CreateIntentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cart, err := h.CartStore.Get(input.CartID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cart not found"})
		return
	}
	if len(cart.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot check out an empty cart"})
		return
	}

	var mc MandateClaims
	parsedToken, err := jwt.ParseWithClaims(input.MandateJWT, &mc, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return verifyKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer("commerce_api"))
	if err != nil || !parsedToken.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mandate token: " + err.Error()})
		return
	}

	var parts []string
	for _, it := range cart.Items {
		parts = append(parts, fmt.Sprintf("%s:%d", it.ProductID, it.Quantity))
	}
	itemsStr := strings.Join(parts, ",")
	hashInput := fmt.Sprintf("%s|%s|%d", cart.ID, itemsStr, cart.TotalPaise)
	sum := sha256.Sum256([]byte(hashInput))
	localHash := hex.EncodeToString(sum[:])

	if mc.CartHash != localHash {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mandate cart hash mismatch"})
		return
	}
	if mc.AmountPa != cart.TotalPaise {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mandate amount mismatch"})
		return
	}

	intent := h.PaymentStore.Create(cart.ID, cart.TotalPaise, input.Currency, mc.MandateID, input.MandateJWT)

	if err := h.PaymentStore.TransitionStatus(intent.ID, models.PaymentStatusRequiresConfirmation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize payment flow"})
		return
	}
	var erro error
	intent, erro = h.PaymentStore.Get(intent.ID)
	if erro != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, intent)

	evt := h.Dispatcher.EventStore.RecordEvent("payment_intent.created", intent)
	h.Dispatcher.Dispatch(evt)

}

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

	// Spend-limit check: ensure the token has enough budget for this charge.
	tokenVal, exists := c.Get("Token")
	if exists {
		token := tokenVal.(*models.AuthToken)
		if err := h.TokenStore.RecordSpend(token.Secret, intent.AmountPaise); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "spend check failed: " + err.Error()})
			return
		}
	}

	// Step 1: Charge the gateway FIRST
	// We must know whether the charge succeeded before emitting any events
	err = h.Gateway.Charge(intent.AmountPaise, intent.Currency)
	if err != nil {
		_ = h.PaymentStore.TransitionStatus(id, models.PaymentStatusFailed)
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "payment gateway charge failed: " + err.Error()})

		if intentFailed, fetchErr := h.PaymentStore.Get(id); fetchErr == nil {
			evt := h.Dispatcher.EventStore.RecordEvent("payment_intent.failed", intentFailed)
			h.Dispatcher.Dispatch(evt)
		}
		return
	}

	// Step 2: Charge succeeded — transition status.
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

	// Step 3: Fetch updated status and respond with final state.
	intent, err = h.PaymentStore.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, intent)

	// Step 4: Emit the success event AFTER responding.
	evt := h.Dispatcher.EventStore.RecordEvent("payment_intent.succeeded", intent)
	h.Dispatcher.Dispatch(evt)
}
