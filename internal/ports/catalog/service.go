package ports

import "context"

type SaveProduct struct {
	Name        string
	Category    string
	Description string
	PriceCents  int64
	Stock       int
}

type SaveHandler interface {
	Handle(ctx context.Context, cmd SaveProduct) error
}
