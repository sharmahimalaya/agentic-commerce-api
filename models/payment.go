package models

import (
	"encoding/json"
	"fmt"
	"time"
)

type PaymentStatus int

const (
	PaymentStatusCreated PaymentStatus = iota
	PaymentStatusRequiresConfirmation
	PaymentStatusSucceeded
	PaymentStatusFailed
)

func (s PaymentStatus) String() string {
	switch s {
	case PaymentStatusCreated:
		return "created"
	case PaymentStatusRequiresConfirmation:
		return "requires_confirmation"
	case PaymentStatusSucceeded:
		return "succeeded"
	case PaymentStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

func (s PaymentStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *PaymentStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "created":
		*s = PaymentStatusCreated
	case "requires_confirmation":
		*s = PaymentStatusRequiresConfirmation
	case "succeeded":
		*s = PaymentStatusSucceeded
	case "failed":
		*s = PaymentStatusFailed
	default:
		return fmt.Errorf("invalid payment status value: %q", str)
	}
	return nil
}

type PaymentIntent struct {
	ID          string        `json:"id"`
	CartID      string        `json:"cart_id"`
	AmountPaise int64         `json:"amount_paise"`
	Currency    string        `json:"currency"`
	Status      PaymentStatus `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}
