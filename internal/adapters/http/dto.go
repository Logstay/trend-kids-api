package http

import (
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
)

type ProductResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	PriceCents  int64     `json:"price_cents"`
	Stock       int       `json:"stock"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

type ListResponse struct {
	Items []ProductResponse `json:"items"`
}

func toResponse(p catalog.Product) ProductResponse {
	return ProductResponse{
		ID: p.UID(), Name: p.Name(), Slug: p.Slug(), Category: string(p.Category()),
		Description: p.Description(), PriceCents: p.PriceCents(), Stock: p.Stock(), Active: p.Active(), CreatedAt: p.CreatedAt(),
	}
}
