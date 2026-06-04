package handlers

import (
	"agentic-commerce/store"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	EventStore *store.EventStore
}

func NewWebhookHandler(es *store.EventStore) *WebhookHandler {
	return &WebhookHandler{
		EventStore: es,
	}
}

type CreateSubscriptionInput struct {
	URL    string   `json:"url" binding:"required,url"`
	Events []string `json:"events" binding:"required,min=1"`
}

func (h *WebhookHandler) CreateSubscription(c *gin.Context) {
	var input CreateSubscriptionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sub, err := h.EventStore.CreateSubscription(input.URL, input.Events)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register webhook"})
		return
	}

	c.JSON(http.StatusOK, sub)
}

func (h *WebhookHandler) ListSubscriptions(c *gin.Context) {
	subs := h.EventStore.ListSubscriptions()
	c.JSON(http.StatusOK, subs)
}

func (h *WebhookHandler) DeleteSubscription(c *gin.Context) {
	id := c.Param("id")
	if err := h.EventStore.DeleteSubscription(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}
