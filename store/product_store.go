package store

import (
	"agentic-commerce/models"
	"errors"
)

var ErrProductNotFound = errors.New("product not found")

type ProductStore struct {
	products []models.Product
}

func NewProductStore() *ProductStore {
	return &ProductStore{
		products: []models.Product{
			{ID: "prod_1", Name: "Parle-G Biscuits", Description: "The iconic Indian glucose biscuit.", PricePaise: 1000, Stock: 100},                   // ₹10.00
			{ID: "prod_2", Name: "Amul Butter 500g", Description: "Utterly butterly delicious pasteurized butter.", PricePaise: 27500, Stock: 50},       // ₹275.00
			{ID: "prod_3", Name: "Tata Salt 1kg", Description: "Desh ka namak, iodized salt.", PricePaise: 2800, Stock: 200},                            // ₹28.00
			{ID: "prod_4", Name: "Haldiram's Bhujia Sev 400g", Description: "Spicy moth bean flour noodles fried snack.", PricePaise: 11000, Stock: 80}, // ₹110.00
			{ID: "prod_5", Name: "Taj Mahal Tea 250g", Description: "Wah Taj! Premium loose leaf black tea.", PricePaise: 21500, Stock: 60},             // ₹215.00
		},
	}
}

func (s *ProductStore) GetAll() []models.Product {
	return s.products
}

func (s *ProductStore) GetById(id string) (models.Product, error) {
	for _, p := range s.products {
		if p.ID == id {
			return p, nil
		}
	}
	return models.Product{}, ErrProductNotFound
}
