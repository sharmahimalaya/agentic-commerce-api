package tests

import (
	"acommerce_api_endpoint/handlers"
	"acommerce_api_endpoint/middleware"
	"acommerce_api_endpoint/models"
	"acommerce_api_endpoint/store"
	"acommerce_api_endpoint/webhook"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
)

const TestPrivatePEM = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDcQcm6JxkGYwFd
xm4QYao4ME78LnaoCcUJ+0pY9UDehNIbmLuwdJ/POfpIRadZlBjHImBftQx2XniG
/snZJY4h/BG1iKmNJnA+U0kwUiovn+rPooxeoU3FpLjHjlR98QI5J2brlp9Xwk0l
crExu+JQKMsw0VXbGvywkh0YSLM2ZbzmzgfDpWwj0uqXbuGjH0hQvnhKSoo5xHQC
O18bbs8q79W2vsLzOlV/zzjrxywbHuX3CIkNlhuv+R4vVuT0RCKaWMgabuwPtZbW
+MZ/ymAmMxj3bsPiw3b0aSV4PuicZ+SuOLj40bev9Llm9F6zRXmYK0oCxQ7mtGe3
9Fqz3w+XAgMBAAECggEADh3SowaseOym2BlxURrMARaU3rQk+2Ic6dCG6aqrrKyl
7Ao0RVFEM3uLdKRaNHM5yfwvDGibKDRSfzxx4rlLIcHOahcciXpkM+pnRMc6AvFk
kvLfvOo+WiN+NZ6uqvUOZ8FZvFGxXBDiRuXR64EXi9GsLDB5KIt96dyDgYdEooDG
RtshE98h4Scrn8wSIFtSWU1BJyvLP1PVBgaG2foqN7aj9vmPx4/TrgbpmouO5RpG
CPvEA0sxbzwaaND3E+2AY91iM4B82ZcYFJCCcsbZe9dl+M/P4H813fvEGsJ2sFIu
biYt6GKunhHheOdFrM0Md+1T5Ry5VijHv4+3pKk9SQKBgQD7um7uzhRpyam09WlI
Y56a9twHDfKDluj0px4ie3GmzEVAew7YDRdQZU3FsTXZpOTcSep87fQZC7f+CFyg
Y0/FznaecX3AJKgHu6v61m1AcHeAgLlfK5JB6BLg7uuyjpRhydzdL4rmsACt13Vn
1y1IdmoanBLI+9vPbadDIpRrEwKBgQDf/qLVpx05WaE1Xuq5tQCk8WdsJrC9c3nb
wtVtg8ocxt/+G0/wrnRpKwG0zP6TAXDh121XIEU5XeNejBEWx+TXlVlkdgJGwCts
Tlu/SsZ8fasR4PQL0J9RNuaWDV0eRvuMdgGxfq53ug6GOHAUAi7or+BcSu1yNokq
YYjO5R817QKBgFpLYYdffIsFv04dyYoh0b6cVghhxF/XPfCkEXck+HtwQlcCzSxK
ZdZ8wAztp/dN4pnyGZ5+bFSfk3wX28HcXb0CdiIXa5gEjhFYDDSJvd6jePorMlMk
+e2SJVNx4DHIWwlIs2TTrOtarqOs6Xw5/xBDCYRJ/6MAVLRvDNRUDxDpAoGBANeW
OFlUh68cEinRGi/1AxK9+eHA91jQXNfkFRFbx9qcmxfyZ6Vp80cJipHev6LzvxbP
BkDWIWpOcDkerI/1gs7vwuMLJbO8385VOL7LlHBbb5w8nAcHG1/KbHK9mAM9JH0T
Uxvnpro7TCFpDo5jb4yrQlDyGMlVrf0pdMhVBA4dAoGBAMaS2BsSS1hk7StJKk7t
AsqrCbxv+YIA1rRkYX0cFJ/4sGxw8PBM0/Q4y86qm4rir55Vp1kK8oT5RobSKVsz
3OeP0WvhDuOquDRw2G/Ifnd/c0Y+oj+YGPGFtKPJjhM49kwm9RdJ/wLDxa8CLXZ2
amF08PXfcJMskGhHUHTq4y/F
-----END PRIVATE KEY-----`

type TestEnv struct {
	Router           *gin.Engine
	ProductStore     *store.ProductStore
	CartStore        *store.CartStore
	PaymentStore     *store.PaymentStore
	TokenStore       *store.TokenStore
	EventStore       *store.EventStore
	IdempotencyStore *middleware.IdempotencyStore
	Dispatcher       *webhook.Dispatcher
	Gateway          *TestGateway
}

type TestGateway struct {
	ShouldFail  bool
	FailError   error
	LastAmount  int64
	LastCurrency string
}

func (g *TestGateway) Charge(amount int64, currency string) error {
	g.LastAmount = amount
	g.LastCurrency = currency
	if g.ShouldFail {
		if g.FailError != nil {
			return g.FailError
		}
		return fmt.Errorf("gateway error")
	}
	return nil
}

var testPrivateKey *rsa.PrivateKey

func init() {
	gin.SetMode(gin.TestMode)
	block, _ := pem.Decode([]byte(TestPrivatePEM))
	if block == nil {
		panic("Failed to parse private key")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	var ok bool
	testPrivateKey, ok = priv.(*rsa.PrivateKey)
	if !ok {
		panic("Not RSA private key")
	}
}

const TestPublicPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3EHJuicZBmMBXcZuEGGq
ODBO/C52qAnFCftKWPVA3oTSG5i7sHSfzzn6SEWnWZQYxyJgX7UMdl54hv7J2SWO
IfwRtYipjSZwPlNJMFIqL5/qz6KMXqFNxaS4x45UffECOSdm65afV8JNJXKxMbvi
UCjLMNFV2xr8sJIdGEizNmW85s4Hw6VsI9Lql27hox9IUL54SkqKOcR0AjtfG27P
Ku/Vtr7C8zpVf88468csGx7l9wiJDZYbr/keL1bk9EQimljIGm7sD7WW1vjGf8pg
JjMY927D4sN29GkleD7onGfkrji4+NG3r/S5ZvRes0V5mCtKAsUO5rRnt/Ras98P
lwIDAQAB
-----END PUBLIC KEY-----`

func SetupTestEnv() *TestEnv {
	productStore := store.NewProductStore()
	cartStore := store.NewCartStore()
	paymentStore := store.NewPaymentStore()
	tokenStore := store.NewTokenStore()
	idempotencyStore := middleware.NewIdempotencyStore()
	eventStore := store.NewEventStore()

	dispatcher := webhook.NewDispatcher(eventStore)
	dispatcher.Start()

	gateway := &TestGateway{}

	productHandler := handlers.NewProductHandler(productStore)
	cartHandler := handlers.NewCartHandler(cartStore, productStore, dispatcher)
	paymentHandler := handlers.NewPaymentHandler(paymentStore, cartStore, tokenStore, gateway, dispatcher, TestPublicPEM)
	tokenHandler := handlers.NewTokenHandler(tokenStore)
	webhookHandler := handlers.NewWebhookHandler(eventStore)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Idempotency(idempotencyStore))

	v1 := r.Group("/v1")
	{
		adminAuth := middleware.RequireAdminKey("test_admin_key")
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

	return &TestEnv{
		Router:           r,
		ProductStore:     productStore,
		CartStore:        cartStore,
		PaymentStore:     paymentStore,
		TokenStore:       tokenStore,
		EventStore:       eventStore,
		IdempotencyStore: idempotencyStore,
		Dispatcher:       dispatcher,
		Gateway:          gateway,
	}
}

type MandateClaims struct {
	MandateID string `json:"mandate_id"`
	CartHash  string `json:"cart_hash"`
	AmountPa  int64  `json:"amount_pa"`
	jwt.RegisteredClaims
}

func GenerateMandateJWT(mandateID, cartHash string, amountPa int64, overrideAlg string) (string, error) {
	claims := MandateClaims{
		MandateID: mandateID,
		CartHash:  cartHash,
		AmountPa:  amountPa,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "commerce_api",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	var token *jwt.Token
	if overrideAlg == "HS256" {
		token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		return token.SignedString([]byte("dummy_key"))
	}
	token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(testPrivateKey)
}
