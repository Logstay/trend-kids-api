package ports

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
)

// MaxSellers: teto razoável para um seletor de WhatsApp que um cliente precisa conseguir ler.
const MaxSellers = 20

type Repository interface {
	// Save e Update devolvem httperr.ConflictError se o WhatsApp já pertence a outra vendedora
	// (garantido pelo banco, então vale mesmo com cadastros simultâneos).
	Save(ctx context.Context, s sellers.Seller) error
	Update(ctx context.Context, s sellers.Seller) error
	// Delete e GetByUID devolvem httperr.NotFoundError se não existir.
	Delete(ctx context.Context, uid string) error
	GetByUID(ctx context.Context, uid string) (sellers.Seller, error)
	// List ordena da mais antiga para a mais nova. onlyActive=true é o que o site público enxerga.
	List(ctx context.Context, onlyActive bool) ([]sellers.Seller, error)
}

type CreateSeller struct {
	Name     string
	WhatsApp string
}

// UpdateSeller é substituição completa (como o PUT).
type UpdateSeller struct {
	UID      string
	Name     string
	WhatsApp string
	Active   bool
}
