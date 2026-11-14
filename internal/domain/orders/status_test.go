package orders_test

import (
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	"github.com/stretchr/testify/assert"
)

func TestCanTransition_OnlyForwardAndCancelBeforeShipping(t *testing.T) {
	allowed := map[[2]orders.Status]bool{
		{orders.StatusNew, orders.StatusPaid}:          true,
		{orders.StatusPaid, orders.StatusShipped}:      true,
		{orders.StatusShipped, orders.StatusDelivered}: true,
		{orders.StatusNew, orders.StatusCancelled}:     true,
		{orders.StatusPaid, orders.StatusCancelled}:    true,
	}
	all := []orders.Status{orders.StatusNew, orders.StatusPaid, orders.StatusShipped, orders.StatusDelivered, orders.StatusCancelled}

	for _, from := range all {
		for _, to := range all {
			want := allowed[[2]orders.Status{from, to}]
			assert.Equal(t, want, orders.CanTransition(from, to), "%s → %s", from, to)
		}
	}
}

func TestCanTransition_UnknownStatusIsNeverAllowed(t *testing.T) {
	assert.False(t, orders.CanTransition("lixo", orders.StatusPaid))
	assert.False(t, orders.CanTransition(orders.StatusNew, "lixo"))
}

func TestParseStatus(t *testing.T) {
	s, ok := orders.ParseStatus("paid")
	assert.True(t, ok)
	assert.Equal(t, orders.StatusPaid, s)

	for _, bad := range []string{"", "PAID", "pago", "refunded"} {
		_, ok := orders.ParseStatus(bad)
		assert.False(t, ok, bad)
	}
}

func TestStatus_IsFinal(t *testing.T) {
	assert.True(t, orders.StatusDelivered.IsFinal())
	assert.True(t, orders.StatusCancelled.IsFinal())
	assert.False(t, orders.StatusNew.IsFinal())
	assert.False(t, orders.StatusShipped.IsFinal())
}
