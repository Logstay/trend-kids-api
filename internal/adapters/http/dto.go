package http

import (
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
)

type ProductResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Category    string `json:"category"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int    `json:"stock"`
	Active      bool   `json:"active"`
	// Image é a URL da capa (a primeira foto; vazia se não houver). Images é a galeria completa, em ordem.
	Image     string          `json:"image"`
	Images    []ImageResponse `json:"images"`
	CreatedAt time.Time       `json:"created_at"`
}

// ImageResponse é uma foto da galeria: a chave identifica a foto nas rotas de remover e reordenar.
type ImageResponse struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

type ListResponse struct {
	Items []ProductResponse `json:"items"`
}

// ImageURLer monta a URL pública de uma foto a partir da chave (o armazenamento sabe onde ela mora).
type ImageURLer interface {
	URL(key string) string
}

type urlFunc func(key string) string

func imageURLs(i ImageURLer) urlFunc {
	if i == nil {
		return func(string) string { return "" }
	}
	return i.URL
}

func toResponse(p catalog.Product, url urlFunc) ProductResponse {
	keys := p.Images()
	images := make([]ImageResponse, 0, len(keys)) // nunca null no JSON
	for _, k := range keys {
		images = append(images, ImageResponse{Key: k, URL: url(k)})
	}
	return ProductResponse{
		Images: images,
		ID:     p.UID(), Name: p.Name(), Slug: p.Slug(), Category: string(p.Category()),
		Description: p.Description(), PriceCents: p.PriceCents(), Stock: p.Stock(), Active: p.Active(),
		Image: url(p.Image()), CreatedAt: p.CreatedAt(),
	}
}
