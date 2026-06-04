package models

type Product struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PricePaise  int64  `json:"price_paise"`
	Stock       int    `json:"stock"`
}
