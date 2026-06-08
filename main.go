package main

import (
	"acommerce_api_endpoint/config"
	"acommerce_api_endpoint/gateway"
	"acommerce_api_endpoint/handlers"
	"acommerce_api_endpoint/middleware"
	"acommerce_api_endpoint/models"
	"acommerce_api_endpoint/store"
	"acommerce_api_endpoint/webhook"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {

	cfg := config.Load()
	productStore := store.NewProductStore()
	cartStore := store.NewCartStore()
	paymentStore := store.NewPaymentStore()
	tokenStore := store.NewTokenStore()
	idempotencyStore := middleware.NewIdempotencyStore()
	eventStore := store.NewEventStore()

	dispatcher := webhook.NewDispatcher(eventStore)
	dispatcher.Start()

	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	idempotencyStore.StartCleanup(cfg.IdempotencyTTL, cleanupCtx)

	mockGateway := &gateway.MockGateway{}

	productHandler := handlers.NewProductHandler(productStore)
	cartHandler := handlers.NewCartHandler(cartStore, productStore, dispatcher)
	paymentHandler := handlers.NewPaymentHandler(paymentStore, cartStore, tokenStore, mockGateway, dispatcher)
	tokenHandler := handlers.NewTokenHandler(tokenStore)
	webhookHandler := handlers.NewWebhookHandler(eventStore)

	r := gin.Default()
	r.Use(middleware.RequestID())
	r.Use(middleware.Idempotency(idempotencyStore))
	v1 := r.Group("/v1")
	{
		adminAuth := middleware.RequireAdminKey(cfg.AdminAPIKey)
		v1.POST("/tokens", adminAuth, tokenHandler.CreateToken)

		v1.GET("/products", middleware.RequireScope(models.ScopeProductsRead, tokenStore), productHandler.ListProducts)
		v1.GET("/products/:id", middleware.RequireScope(models.ScopeProductsRead, tokenStore), productHandler.GetProduct)

		v1.POST("/carts", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), cartHandler.CreateCart)
		v1.GET("/carts/:id", middleware.RequireScope(models.ScopeCartsRead, tokenStore), cartHandler.GetCart)
		v1.POST("/carts/:id/items", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), cartHandler.AddItem)
		v1.PUT("/carts/:id/items/:productId", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), cartHandler.UpdateItem)
		v1.DELETE("/carts/:id/items/:productId", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), cartHandler.RemoveItem)

		v1.POST("/payment-intents", middleware.RequireScope(models.ScopePaymentsWrite, tokenStore), paymentHandler.CreateIntent)
		v1.GET("/payment-intents/:id", middleware.RequireScope(models.ScopePaymentsRead, tokenStore), paymentHandler.GetIntent)
		v1.POST("/payment-intents/:id/confirm", middleware.RequireScope(models.ScopePaymentsWrite, tokenStore), paymentHandler.ConfirmIntent)

		v1.POST("/webhooks", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), webhookHandler.CreateSubscription)
		v1.GET("/webhooks", middleware.RequireScope(models.ScopeCartsRead, tokenStore), webhookHandler.ListSubscriptions)
		v1.DELETE("/webhooks/:id", middleware.RequireScope(models.ScopeCartsWrite, tokenStore), webhookHandler.DeleteSubscription)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Println("Starting API on port " + cfg.Port + "...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}
	dispatcher.Stop()
	log.Println("Server exiting")
}
