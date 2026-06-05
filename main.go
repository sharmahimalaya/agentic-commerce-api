package main

import (
	"agentic-commerce/gateway"
	"agentic-commerce/handlers"
	"agentic-commerce/middleware"
	"agentic-commerce/models"
	"agentic-commerce/store"
	"agentic-commerce/webhook"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	productStore := store.NewProductStore()
	cartStore := store.NewCartStore()
	paymentStore := store.NewPaymentStore()
	tokenStore := store.NewTokenStore()
	idempotencyStore := middleware.NewIdempotencyStore()
	eventStore := store.NewEventStore()

	dispatcher := webhook.NewDispatcher(eventStore)
	dispatcher.Start()

	mockGateway := &gateway.MockGateway{}

	productHandler := handlers.NewProductHandler(productStore)
	cartHandler := handlers.NewCartHandler(cartStore, productStore, dispatcher)
	paymentHandler := handlers.NewPaymentHandler(paymentStore, cartStore, tokenStore, mockGateway, dispatcher)
	tokenHandler := handlers.NewTokenHandler(tokenStore)
	webhookHandler := handlers.NewWebhookHandler(eventStore)

	r := gin.Default()
	r.Use(middleware.RequestID())
	r.Use(middleware.Idempotency(idempotencyStore))

	r.POST("/tokens", tokenHandler.CreateToken)

	r.GET("/products", middleware.RequireScope(models.ScopeProductsRead, tokenStore), productHandler.ListProducts)
	r.GET("/products/:id", middleware.RequireScope(models.ScopeProductsRead, tokenStore), productHandler.GetProduct)

	r.POST("/carts", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), cartHandler.CreateCart)
	r.GET("/carts/:id", middleware.RequireScope(models.ScopeCartsRead, tokenStore), cartHandler.GetCart)
	r.POST("/carts/:id/items", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), cartHandler.AddItem)
	r.PUT("/carts/:id/items/:itemId", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), cartHandler.UpdateItem)
	r.DELETE("/carts/:id/items/:itemId", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), cartHandler.RemoveItem)

	r.POST("/payment-intents", middleware.RequireScope(models.ScopePaymentsWrite, tokenStore), paymentHandler.CreateIntent)
	r.GET("/payment-intents/:id", middleware.RequireScope(models.ScopePaymentsRead, tokenStore), paymentHandler.GetIntent)
	r.POST("/payment-intents/:id/confirm", middleware.RequireScope(models.ScopePaymentsWrite, tokenStore), paymentHandler.ConfirmIntent)

	r.POST("/webhooks", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), webhookHandler.CreateSubscription)
	r.GET("/webhooks", middleware.RequireScope(models.ScopeCartsRead, tokenStore), webhookHandler.ListSubscriptions)
	r.DELETE("/webhooks/:id", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), webhookHandler.DeleteSubscription)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Println("Starting API on port 8080...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}
	dispatcher.Stop()
	log.Println("Server exiting")
}
