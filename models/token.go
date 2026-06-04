package models

import "time"

type Scope string

const (
	ScopeProductsRead  Scope = "products:read"
	ScopeCartsRead     Scope = "carts:read"
	ScopeCartsWrite    Scope = "carts:write"
	ScopePaymentsRead  Scope = "payments:read"
	ScopePaymentsWrite Scope = "payments:write"
)

type AuthToken struct {
	ID                string    `json:"id"`
	Secret            string    `json:"secret,omitempty"`
	Scopes            []Scope   `json:"scopes"`
	SpendLimitPaise   int64     `json:"spend_limit_paise"`
	CurrentSpendPaise int64     `json:"current_spend_paise"`
	ExpiresAt         time.Time `json:"expires_at"`
	CreatedAt         time.Time `json:"created_at"`
}

func (t *AuthToken) HasScope(required Scope) bool {
	for _, s := range t.Scopes {
		if s == required {
			return true
		}
	}
	return false
}
