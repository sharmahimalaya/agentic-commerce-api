package models

import "time"

type CartItem struct {
	ProductID  string `json:"product_id"`
	Quantity   int    `json:"quantity"`
	PricePaise int64  `json:"price_paise"`
}

type Cart struct {
	ID         string     `json:"id"`
	Items      []CartItem `json:"items"`
	TotalPaise int64      `json:"total_paise"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (c *Cart) FindItemIndex(productID string) int {
	for i, item := range c.Items {
		if item.ProductID == productID {
			return i
		}
	}
	return -1
}
