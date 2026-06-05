package handlers

import (
	"agentic-commerce/models"
	"agentic-commerce/store"
	"agentic-commerce/webhook"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CartHandler struct {
	CartStore    *store.CartStore
	ProductStore *store.ProductStore
	Dispatcher   *webhook.Dispatcher
}

func NewCartHandler(cs *store.CartStore, ps *store.ProductStore, d *webhook.Dispatcher) *CartHandler {
	return &CartHandler{CartStore: cs, ProductStore: ps, Dispatcher: d}
}

func (h *CartHandler) CreateCart(c *gin.Context) {
	cart := h.CartStore.Create()

	evt := h.Dispatcher.EventStore.RecordEvent("cart.created", cart)
	h.Dispatcher.Dispatch(evt)
	c.JSON(http.StatusCreated, cart)
}

func (h *CartHandler) GetCart(c *gin.Context) {
	id := c.Param("id")
	cart, err := h.CartStore.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cart)
}

type AddItemInput struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

func (h *CartHandler) AddItem(c *gin.Context) {
	cartID := c.Param("id")
	cart, err := h.CartStore.Get(cartID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cart not found"})
		return
	}
	var input AddItemInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := h.ProductStore.GetById(input.ProductID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	if product.Stock < input.Quantity {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stock available"})
		return
	}
	foundIndex := cart.FindItemIndex(input.ProductID)
	if foundIndex >= 0 {
		cart.Items[foundIndex].Quantity += input.Quantity
	} else {
		newItem := models.CartItem{
			ProductID:  product.ID,
			Quantity:   input.Quantity,
			PricePaise: product.PricePaise,
		}
		cart.Items = append(cart.Items, newItem)
	}

	h.CartStore.Save(cart)

	c.JSON(http.StatusOK, cart)
}

type UpdateItemInput struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}

func (h *CartHandler) UpdateItem(c *gin.Context) {
	cartID := c.Param("id")
	productID := c.Param("itemId")

	cart, err := h.CartStore.Get(cartID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cart not found"})
		return
	}

	var input UpdateItemInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	foundIndex := cart.FindItemIndex(productID)

	if foundIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found in cart"})
		return
	}

	product, err := h.ProductStore.GetById(productID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "product no longer exists"})
		return
	}
	if product.Stock < input.Quantity {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stock available"})
		return
	}

	cart.Items[foundIndex].Quantity = input.Quantity
	h.CartStore.Save(cart)

	c.JSON(http.StatusOK, cart)
}

func (h *CartHandler) RemoveItem(c *gin.Context) {
	cartID := c.Param("id")
	productID := c.Param("itemId")

	cart, err := h.CartStore.Get(cartID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cart not found"})
		return
	}
	foundIndex := cart.FindItemIndex(productID)

	if foundIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found in cart"})
		return
	}
	cart.Items = append(cart.Items[:foundIndex], cart.Items[foundIndex+1:]...)
	h.CartStore.Save(cart)
	c.JSON(http.StatusOK, cart)
}
