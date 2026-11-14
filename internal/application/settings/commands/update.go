package commands

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/settings"
	ports "github.com/Logstay/trend-kids-api/internal/ports/settings"
)

type UpdateHandler struct {
	Repository ports.Repository
}

// Handle valida tudo antes de gravar: se qualquer campo for inválido, nada muda.
func (h UpdateHandler) Handle(ctx context.Context, cmd ports.UpdateSettings) (settings.Settings, error) {
	f := settings.NewSettingsFactory()
	for _, set := range []func() error{
		func() error { return f.Name(cmd.Name) },
		func() error { return f.City(cmd.City) },
		func() error { return f.Description(cmd.Description) },
		func() error { return f.WhatsApp(cmd.WhatsApp) },
		func() error { return f.Phone(cmd.Phone) },
		func() error { return f.Instagram(cmd.Instagram) },
	} {
		if err := set(); err != nil {
			return nil, err
		}
	}

	built, err := f.Build()
	if err != nil {
		return nil, err
	}
	if err := h.Repository.Update(ctx, built, cmd.AdminUID); err != nil {
		return nil, err
	}
	return h.Repository.Get(ctx) // relê para devolver também a data gravada pelo banco
}
