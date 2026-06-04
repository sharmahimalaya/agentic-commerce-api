package handlers

import (
	"agentic-commerce/models"
	"agentic-commerce/store"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type TokenHandler struct {
	TokenStore *store.TokenStore
}

func NewTokenHandler(ts *store.TokenStore) *TokenHandler {
	return &TokenHandler{TokenStore: ts}
}

type CreateTokenInput struct {
	Scopes          []models.Scope `json:"scopes" binding:"required"`
	SpendLimitPaise int64          `json:"spend_limit_paise"`
	DurationHours   int            `json:"duration_hours" binding:"required"`
}

func (h *TokenHandler) CreateToken(c *gin.Context) {
	var input CreateTokenInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	duration := time.Duration(input.DurationHours) * time.Hour
	token, err := h.TokenStore.Create(input.Scopes, input.SpendLimitPaise, duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, token)
}
