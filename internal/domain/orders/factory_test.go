package orders_test

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validFactory(t *testing.T) orders.OrderFactory {
	t.Helper()
	f := orders.NewOrderFactory()
	require.NoError(t, f.Customer("Maria Silva", "(83) 99999-1234", "Manaíra"))
	require.NoError(t, f.AddItem("p1", "Classic Clog Rosa", 18990, 2))
	return f
}

func TestOrder_TotalsComeFromItemsAndShippingPolicy(t *testing.T) {
	f := validFactory(t)
	require.NoError(t, f.AddItem("p2", "Kit 5 Charms", 4990, 1))

	o, err := f.Build(orders.ShippingPolicy{FlatCents: 1500})

	require.NoError(t, err)
	assert.Equal(t, int64(2*18990+4990), o.SubtotalCents())
	assert.Equal(t, int64(1500), o.ShippingCents())
	assert.Equal(t, o.SubtotalCents()+1500, o.TotalCents())
	assert.Equal(t, orders.StatusNew, o.Status())
	assert.Len(t, o.Items(), 2)
}

func TestOrder_CodeIsHumanFriendlyWithoutAmbiguousChars(t *testing.T) {
	re := regexp.MustCompile(`^TK[2-9A-HJKMNP-Z]{8}$`)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		o, err := validFactory(t).Build(orders.ShippingPolicy{})
		require.NoError(t, err)
		assert.Regexp(t, re, o.Code())
		seen[o.Code()] = true
	}
	assert.Len(t, seen, 200, "códigos não se repetem")
}

func TestShippingPolicy(t *testing.T) {
	p := orders.ShippingPolicy{FlatCents: 1500, FreeAboveCents: 29900}
	assert.Equal(t, int64(1500), p.CostFor(29899))
	assert.Equal(t, int64(0), p.CostFor(29900), "grátis a partir do valor")
	assert.Equal(t, int64(0), p.CostFor(50000))
	assert.Equal(t, int64(0), orders.ShippingPolicy{}.CostFor(100), "padrão: sem frete até a regra ser definida")
	assert.Equal(t, int64(1500), orders.ShippingPolicy{FlatCents: 1500}.CostFor(1_000_000), "sem limite de grátis, sempre cobra")
}

func TestCustomer_NormalizesPhone(t *testing.T) {
	for in, want := range map[string]string{
		"(83) 99999-1234":   "83999991234",
		"+55 83 99999-1234": "83999991234",
		"83 3222-1234":      "8332221234",
		"5583999991234":     "83999991234",
	} {
		f := orders.NewOrderFactory()
		require.NoError(t, f.Customer("Maria", in, "Manaíra"), in)
		require.NoError(t, f.AddItem("p", "x", 100, 1))
		o, _ := f.Build(orders.ShippingPolicy{})
		assert.Equal(t, want, o.CustomerPhone(), in)
	}
}

func TestCustomer_RejectsInvalidData(t *testing.T) {
	tests := map[string][3]string{
		"nome vazio":            {" ", "83999991234", "Manaíra"},
		"nome de 1 letra":       {"M", "83999991234", "Manaíra"},
		"nome gigante":          {string(make([]rune, 101)), "83999991234", "Manaíra"},
		"bairro vazio":          {"Maria", "83999991234", "  "},
		"telefone curto":        {"Maria", "123", "Manaíra"},
		"telefone com letras":   {"Maria", "83 9999x-1234", "Manaíra"},
		"DDD inexistente":       {"Maria", "00999991234", "Manaíra"},
		"celular sem o 9":       {"Maria", "83899991234", "Manaíra"},
		"telefone longo demais": {"Maria", "558399999123456", "Manaíra"},
	}
	for name, c := range tests {
		err := orders.NewOrderFactory().Customer(c[0], c[1], c[2])
		var req httperr.RequiredError
		assert.True(t, errors.As(err, &req), name)
	}
}

func TestAddItem_ValidatesAndMergesDuplicates(t *testing.T) {
	f := orders.NewOrderFactory()
	require.NoError(t, f.AddItem("p1", "Classic", 1000, 2))
	require.NoError(t, f.AddItem("p1", "Classic", 1000, 3))
	require.NoError(t, f.Customer("Maria", "83999991234", "Manaíra"))

	o, err := f.Build(orders.ShippingPolicy{})

	require.NoError(t, err)
	require.Len(t, o.Items(), 1)
	assert.Equal(t, 5, o.Items()[0].Quantity)
}

func TestAddItem_Limits(t *testing.T) {
	f := orders.NewOrderFactory()
	assert.Error(t, f.AddItem("p", "x", 1000, 0), "quantidade zero")
	assert.Error(t, f.AddItem("p", "x", 1000, orders.MaxQuantity+1), "quantidade acima do limite")
	assert.Error(t, f.AddItem("p", "x", 0, 1), "preço zero")
	assert.Error(t, f.AddItem("p", " ", 1000, 1), "nome vazio")
	assert.Error(t, f.AddItem("", "x", 1000, 1), "produto vazio")

	require.NoError(t, f.AddItem("p", "x", 1000, orders.MaxQuantity))
	assert.Error(t, f.AddItem("p", "x", 1000, 1), "somar duplicado passa do limite")

	for i := 1; i < orders.MaxLines; i++ {
		require.NoError(t, f.AddItem(fmt.Sprintf("linha-%d", i), "x", 1000, 1))
	}
	assert.Error(t, f.AddItem("zz", "x", 1000, 1), "linhas demais")
}

func TestBuild_RequiresCustomerAndItems(t *testing.T) {
	onlyItem := orders.NewOrderFactory()
	require.NoError(t, onlyItem.AddItem("p", "x", 100, 1))
	_, err := onlyItem.Build(orders.ShippingPolicy{})
	assert.Error(t, err, "sem cliente")

	onlyCustomer := orders.NewOrderFactory()
	require.NoError(t, onlyCustomer.Customer("Maria", "83999991234", "Manaíra"))
	_, err = onlyCustomer.Build(orders.ShippingPolicy{})
	assert.Error(t, err, "sem itens")
}
