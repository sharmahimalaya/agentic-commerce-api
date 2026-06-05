package gateway

import "log"

type MockGateway struct{}

func (g *MockGateway) Charge(amount int64, currency string) error {
	log.Printf("[MOCK GATEWAY] Charging %d %s", amount, currency)
	return nil
}
