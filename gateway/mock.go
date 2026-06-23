package gateway

import "log/slog"


// MockGateway is a fake payment processor for testing and development.
// It doesn't actually hit any banks, it just prints a nice JSON log and says "success"!
type MockGateway struct{}

// Charge pretends to process a payment and always returns success (nil).
func (g *MockGateway) Charge(amount int64, currency string) error {
	slog.Info("Charging via Mock Gateway", slog.Int64("amount", amount), slog.String("currency", currency))
	return nil
}

