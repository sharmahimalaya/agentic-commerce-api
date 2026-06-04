package handlers

import (
	"agentic-commerce/store"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ProductHandler struct {
	Store *store.ProductStore
}

func NewProductHandler(s *store.ProductStore) *ProductHandler {
	return &ProductHandler{Store: s}
}

func (h *ProductHandler) ListProducts(c *gin.Context) {
	products := h.Store.GetAll()
	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) GetProduct(c *gin.Context) {
	id := c.Param("id")

	product, err := h.Store.GetById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, product)
}
