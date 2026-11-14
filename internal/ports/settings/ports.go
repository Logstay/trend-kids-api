package ports

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/settings"
)

// Repository guarda as configurações da loja (uma linha só; sempre existe, com valores padrão).
type Repository interface {
	Get(ctx context.Context) (settings.Settings, error)
	// Update substitui tudo e registra quem alterou.
	Update(ctx context.Context, s settings.Settings, adminUID string) error
}

// UpdateSettings é uma substituição completa (como o PUT): campo vazio de contato = remover o contato.
type UpdateSettings struct {
	Name        string
	City        string
	Description string
	WhatsApp    string
	Phone       string
	Instagram   string
	AdminUID    string
}
