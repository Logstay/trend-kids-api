package commands

import (
	"context"
	"fmt"

	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
	ports "github.com/Logstay/trend-kids-api/internal/ports/sellers"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type CreateHandler struct {
	Repository ports.Repository
}

func (h CreateHandler) Handle(ctx context.Context, cmd ports.CreateSeller) (sellers.Seller, error) {
	f := sellers.NewSellerFactory()
	if err := f.Name(cmd.Name); err != nil {
		return nil, err
	}
	if err := f.WhatsApp(cmd.WhatsApp); err != nil {
		return nil, err
	}
	built, err := f.Build()
	if err != nil {
		return nil, err
	}

	all, err := h.Repository.List(ctx, false)
	if err != nil {
		return nil, err
	}
	if len(all) >= ports.MaxSellers {
		return nil, httperr.ConflictError(fmt.Sprintf("limite de %d vendedoras atingido", ports.MaxSellers))
	}

	// A duplicidade de WhatsApp é garantida pelo banco (índice único), então vale mesmo com cadastros simultâneos.
	if err := h.Repository.Save(ctx, built); err != nil {
		return nil, err
	}
	return h.Repository.GetByUID(ctx, built.UID()) // relê para devolver também a data gravada pelo banco
}
