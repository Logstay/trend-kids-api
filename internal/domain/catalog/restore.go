package catalog

import "time"

// Snapshot são os dados de um produto já persistido.
type Snapshot struct {
	UID         string
	Name        string
	Slug        string
	Category    string
	Description string
	PriceCents  int64
	Stock       int
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Restore reconstrói um produto vindo do banco, sem revalidar nem alterar nada.
func Restore(s Snapshot) Product {
	return &product{
		uid: s.UID, name: s.Name, slug: s.Slug, category: Category(s.Category),
		description: s.Description, priceCents: s.PriceCents, stock: s.Stock,
		active: s.Active, createdAt: s.CreatedAt, updatedAt: s.UpdatedAt,
	}
}
