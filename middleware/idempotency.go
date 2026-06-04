package middleware

import (
	"bytes"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

type SavedResponse struct {
	Status int
	Body   []byte
	Header http.Header
}

type IdempotencyStore struct {
	mu    sync.RWMutex
	cache map[string]SavedResponse
}

func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{
		cache: make(map[string]SavedResponse),
	}
}

type responseWriterWrapper struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func Idempotency(store *IdempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}

		store.mu.RLock()
		cached, exists := store.cache[key]

		store.mu.RUnlock()

		if exists {
			for k, values := range cached.Header {
				for _, val := range values {
					c.Writer.Header().Add(k, val)
				}
			}
			c.Data(cached.Status, "application/json", cached.Body)
			c.Abort()
			return
		}

		buf := &bytes.Buffer{}
		originalWriter := c.Writer
		wrapper := &responseWriterWrapper{ResponseWriter: originalWriter, body: buf}

		c.Writer = wrapper

		c.Next()

		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			store.mu.Lock()

			store.cache[key] = SavedResponse{
				Status: c.Writer.Status(),
				Body:   buf.Bytes(),
				Header: originalWriter.Header().Clone(),
			}
			store.mu.Unlock()
		}
	}
}
