package handlers

import (
	"agentic-commerce/models"
	"agentic-commerce/store"
	"agentic-commerce/webhook"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PaymentGateway interface {
	Charge(amount int64, currency string) error
}

type PaymentHandler struct {
	PaymentStore *store.PaymentStore
	CartStore    *store.CartStore
	TokenStore   *store.TokenStore
	Gateway      PaymentGateway
	Dispatcher   *webhook.Dispatcher
}

func NewPaymentHandler(ps *store.PaymentStore, cs *store.CartStore, ts *store.TokenStore, pg PaymentGateway, d *webhook.Dispatcher) *PaymentHandler {
	return &PaymentHandler{
		PaymentStore: ps,
		CartStore:    cs,
		TokenStore:   ts,
		Gateway:      pg,
		Dispatcher:   d,
	}
}

type CreateIntentInput struct {
	CartID   string `json:"cart_id" binding:"required"`
	Currency string `json:"currency" binding:"required,oneof=INR USD EUR"`
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

	intent := h.PaymentStore.Create(cart.ID, cart.TotalPaise, input.Currency)

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
	_, exists := c.Get("Token")
	if exists {
		secret := c.MustGet("TokenSecret").(string)
		if err := h.TokenStore.RecordSpend(secret, intent.AmountPaise); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "spend check failed: " + err.Error()})
			return
		}
	}

	// Step 1: Charge the gateway FIRST.
	// We must know whether the charge succeeded before emitting any events.
	err = h.Gateway.Charge(intent.AmountPaise, intent.Currency)
	if err != nil {
		// Gateway failed — transition to "failed" and emit the failure event.
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
