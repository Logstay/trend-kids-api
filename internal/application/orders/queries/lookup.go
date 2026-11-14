package queries

import (
	"context"
	"crypto/subtle"
	goerrors "errors"
	"strings"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type LookupHandler struct {
	Orders ports.Repository
}

var errNotFound = httperr.NotFoundError("pedido não encontrado")

// Handle exige código E telefone. Código inexistente, telefone errado e telefone malformado dão
// exatamente a mesma resposta, para ninguém descobrir quais códigos existem.
func (h LookupHandler) Handle(ctx context.Context, q ports.Lookup) (orders.Order, error) {
	phone, ok := orders.NormalizePhone(q.Phone)
	if !ok {
		return nil, errNotFound
	}

	order, err := h.Orders.GetByCode(ctx, strings.ToUpper(strings.TrimSpace(q.Code)))
	if err != nil {
		var nf httperr.NotFoundError
		if goerrors.As(err, &nf) {
			return nil, errNotFound
		}
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(order.CustomerPhone()), []byte(phone)) != 1 {
		return nil, errNotFound
	}
	return order, nil
}
