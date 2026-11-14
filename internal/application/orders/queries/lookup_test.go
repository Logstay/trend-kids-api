package queries_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/orders/queries"
	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	"github.com/Logstay/trend-kids-api/internal/ports/orders/orderstest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seeded(t *testing.T) (queries.LookupHandler, orders.Order) {
	t.Helper()
	f := orders.NewOrderFactory()
	require.NoError(t, f.Customer("Maria Silva", "83999991234", "Manaíra"))
	require.NoError(t, f.AddItem("p1", "Classic Clog Rosa", 18990, 1))
	o, err := f.Build(orders.ShippingPolicy{})
	require.NoError(t, err)
	return queries.LookupHandler{Orders: &orderstest.Repo{Orders: []orders.Order{o}}}, o
}

func TestLookup_ReturnsOrderForMatchingCodeAndPhone(t *testing.T) {
	h, o := seeded(t)

	got, err := h.Handle(context.Background(), ports.Lookup{Code: o.Code(), Phone: "(83) 99999-1234"})

	require.NoError(t, err)
	assert.Equal(t, o.UID(), got.UID())
}

func TestLookup_CodeIsCaseInsensitive(t *testing.T) {
	h, o := seeded(t)
	_, err := h.Handle(context.Background(), ports.Lookup{Code: "  " + lower(o.Code()) + " ", Phone: "83999991234"})
	assert.NoError(t, err)
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// Quem chuta códigos não pode descobrir quais existem.
func TestLookup_WrongPhoneUnknownCodeAndBadPhoneLookIdentical(t *testing.T) {
	h, o := seeded(t)
	ctx := context.Background()

	_, wrongPhone := h.Handle(ctx, ports.Lookup{Code: o.Code(), Phone: "83988887777"})
	_, unknownCode := h.Handle(ctx, ports.Lookup{Code: "TK22222222", Phone: "83999991234"})
	_, badPhone := h.Handle(ctx, ports.Lookup{Code: o.Code(), Phone: "lixo"})

	for _, err := range []error{wrongPhone, unknownCode, badPhone} {
		var nf httperr.NotFoundError
		require.True(t, errors.As(err, &nf))
	}
	assert.Equal(t, wrongPhone.Error(), unknownCode.Error())
	assert.Equal(t, wrongPhone.Error(), badPhone.Error())
}
