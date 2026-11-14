package ports

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
)

type Sort string

const (
	SortNewest    Sort = "newest"
	SortPriceAsc  Sort = "price_asc"
	SortPriceDesc Sort = "price_desc"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Filter struct {
	Category string
	Query    string
	Sort     Sort
	Limit    int
	Offset   int
}

// Repository é o contrato de persistência do catálogo.
// Get devolve httperr.NotFoundError quando o produto não existe.
type Repository interface {
	Save(ctx context.Context, product catalog.Product) error
	GetBySlug(ctx context.Context, slug string) (catalog.Product, error)
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
	List(ctx context.Context, filter Filter) ([]catalog.Product, error)
}
