package handlers

import (
	"acommerce_api_endpoint/models"
	"acommerce_api_endpoint/store"
	"acommerce_api_endpoint/webhook"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CartHandler is the coordinator for everything related to shopping carts.
// It ties together the store layer, product catalog validation, and webhook dispatching.
type CartHandler struct {
	CartStore    store.CartStorer
	ProductStore *store.ProductStore
	Dispatcher   *webhook.Dispatcher
}

// NewCartHandler creates a cart handler instance.
func NewCartHandler(cs store.CartStorer, ps *store.ProductStore, d *webhook.Dispatcher) *CartHandler {
	return &CartHandler{CartStore: cs, ProductStore: ps, Dispatcher: d}
}

// CreateCart handles POST /v1/carts.
// It initializes an empty cart, emits a 'cart.created' webhook event, and returns the cart object.
func (h *CartHandler) CreateCart(c *gin.Context) {
	cart := h.CartStore.Create()

	// Notify webhook subscribers that a new cart has been initialized
	evt := h.Dispatcher.EventStore.RecordEvent("cart.created", cart)
	h.Dispatcher.Dispatch(evt)
	c.JSON(http.StatusCreated, cart)
}

// GetCart handles GET /v1/carts/:id.
// It retrieves the cart by its ID from the store. If not found, returns a 404.
func (h *CartHandler) GetCart(c *gin.Context) {
	cartID := c.Param("id")
	cart, err := h.CartStore.Get(cartID)
	if err != nil {
		if errors.Is(err, store.ErrCartNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cart)
}

// AddItemInput represents the payload schema when adding an item to the cart.
type AddItemInput struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

// AddItem handles POST /v1/carts/:id/items.
// It checks if the product exists, verifies we have enough stock,
// and either adds a new item or updates the quantity of an existing one.
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

	// Validate the product actually exists in our store
	product, err := h.ProductStore.GetById(input.ProductID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	// Make sure we aren't selling things we don't have
	if product.Stock < input.Quantity {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stock available"})
		return
	}
	foundIndex := cart.FindItemIndex(input.ProductID)
	if foundIndex >= 0 {
		// Item is already in the cart, let's just add to the quantity
		cart.Items[foundIndex].Quantity += input.Quantity
	} else {
		// New item! Let's append it to the items list
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

// UpdateItemInput is the payload schema when changing the quantity of a cart item.
type UpdateItemInput struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}

// UpdateItem handles PUT /v1/carts/:id/items/:productId.
// It sets the quantity of a specific item in the cart, checking stock availability.
func (h *CartHandler) UpdateItem(c *gin.Context) {
	cartID := c.Param("id")
	productID := c.Param("productId")

	cart, err := h.CartStore.Get(cartID)
	if err != nil {
		if errors.Is(err, store.ErrCartNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

// RemoveItem handles DELETE /v1/carts/:id/items/:productId.
// It kicks an item out of the cart by slicing it out.
func (h *CartHandler) RemoveItem(c *gin.Context) {
	cartID := c.Param("id")
	productID := c.Param("productId")

	cart, err := h.CartStore.Get(cartID)
	if err != nil {
		if errors.Is(err, store.ErrCartNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	foundIndex := cart.FindItemIndex(productID)

	if foundIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found in cart"})
		return
	}
	// Chop the item out of the slice of cart items
	cart.Items = append(cart.Items[:foundIndex], cart.Items[foundIndex+1:]...)
	h.CartStore.Save(cart)
	c.JSON(http.StatusOK, cart)
}

