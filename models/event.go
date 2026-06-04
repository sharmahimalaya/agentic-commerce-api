package models

import "time"

type WebhookSubscription struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Events        []string  `json:"events"`
	SigningSecret string    `json:"signing_secret,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type WebhookEvent struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	CreatedAt time.Time   `json:"created_at"`
}
