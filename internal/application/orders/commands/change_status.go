package commands

import (
	"context"
	"strings"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type ChangeStatusHandler struct {
	Orders ports.AdminRepository
}

// Handle valida só o formato do pedido de mudança. A regra de transição (e a devolução de estoque
// ao cancelar) é aplicada pelo repositório dentro da transação, com a linha do pedido travada.
func (h ChangeStatusHandler) Handle(ctx context.Context, cmd ports.ChangeStatusCommand) (orders.Order, error) {
	to, ok := orders.ParseStatus(cmd.Status)
	if !ok {
		return nil, httperr.RequiredError("status inválido")
	}
	return h.Orders.UpdateStatus(ctx, ports.ChangeStatus{
		Code:     strings.ToUpper(strings.TrimSpace(cmd.Code)),
		To:       to,
		AdminUID: cmd.AdminUID,
	})
}
