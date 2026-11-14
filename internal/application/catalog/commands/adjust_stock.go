package commands

import (
	"context"
	"strings"

	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type AdjustStockHandler struct {
	Repository ports.Repository
}

// Handle devolve o novo saldo. A regra "saldo nunca negativo" é aplicada de forma atômica pelo repositório.
func (h AdjustStockHandler) Handle(ctx context.Context, cmd ports.AdjustStock) (int, error) {
	reason := strings.TrimSpace(cmd.Reason)
	switch {
	case cmd.Delta == 0:
		return 0, httperr.RequiredError("a quantidade do ajuste não pode ser zero")
	case reason == "":
		return 0, httperr.RequiredError("o motivo do ajuste é obrigatório")
	case len(reason) > ports.MaxReasonLen:
		return 0, httperr.RequiredError("motivo do ajuste muito longo")
	}

	return h.Repository.AdjustStock(ctx, ports.StockMovement{
		ProductUID: cmd.UID, Delta: cmd.Delta, Reason: reason, AdminUID: cmd.AdminUID,
	})
}
