package commands_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/sellers/commands"
	ports "github.com/Logstay/trend-kids-api/internal/ports/sellers"
	"github.com/Logstay/trend-kids-api/internal/ports/sellers/sellerstest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func create(t *testing.T, repo *sellerstest.Repo, name, whatsapp string) string {
	t.Helper()
	s, err := commands.CreateHandler{Repository: repo}.Handle(context.Background(), ports.CreateSeller{Name: name, WhatsApp: whatsapp})
	require.NoError(t, err)
	return s.UID()
}

func TestCreate_StoresActiveSellerWithNormalizedNumber(t *testing.T) {
	repo := &sellerstest.Repo{}

	s, err := commands.CreateHandler{Repository: repo}.Handle(context.Background(), ports.CreateSeller{Name: "Ana", WhatsApp: "(83) 99999-1234"})

	require.NoError(t, err)
	assert.Equal(t, "83999991234", s.WhatsApp())
	assert.True(t, s.Active())
	assert.Len(t, repo.Sellers, 1)
}

func TestCreate_InvalidInputSavesNothing(t *testing.T) {
	repo := &sellerstest.Repo{}
	for name, cmd := range map[string]ports.CreateSeller{
		"sem nome":      {Name: " ", WhatsApp: "83999991234"},
		"sem whatsapp":  {Name: "Ana", WhatsApp: ""},
		"whatsapp ruim": {Name: "Ana", WhatsApp: "123"},
	} {
		_, err := commands.CreateHandler{Repository: repo}.Handle(context.Background(), cmd)
		var req httperr.RequiredError
		assert.True(t, errors.As(err, &req), name)
	}
	assert.Empty(t, repo.Sellers)
}

func TestCreate_RejectsDuplicateWhatsAppInAnyFormat(t *testing.T) {
	repo := &sellerstest.Repo{}
	create(t, repo, "Ana", "(83) 99999-1234")

	_, err := commands.CreateHandler{Repository: repo}.Handle(context.Background(), ports.CreateSeller{Name: "Bia", WhatsApp: "+55 83 99999-1234"})

	var conflict httperr.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Len(t, repo.Sellers, 1)
}

func TestCreate_StopsAtTheLimit(t *testing.T) {
	repo := &sellerstest.Repo{}
	for i := 0; i < ports.MaxSellers; i++ {
		create(t, repo, fmt.Sprintf("Vendedora %02d", i), fmt.Sprintf("8399999%04d", i))
	}

	_, err := commands.CreateHandler{Repository: repo}.Handle(context.Background(), ports.CreateSeller{Name: "Uma a mais", WhatsApp: "83988887777"})

	var conflict httperr.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Len(t, repo.Sellers, ports.MaxSellers)
}

func TestUpdate_ChangesFieldsAndCanDeactivate(t *testing.T) {
	repo := &sellerstest.Repo{}
	uid := create(t, repo, "Ana", "83999991234")

	got, err := commands.UpdateHandler{Repository: repo}.Handle(context.Background(),
		ports.UpdateSeller{UID: uid, Name: "Ana Paula", WhatsApp: "(83) 98888-7766", Active: false})

	require.NoError(t, err)
	assert.Equal(t, "Ana Paula", got.Name())
	assert.Equal(t, "83988887766", got.WhatsApp())
	assert.False(t, got.Active())
	assert.Equal(t, uid, got.UID())
}

func TestUpdate_CannotTakeAnotherSellersNumberButKeepsItsOwn(t *testing.T) {
	repo := &sellerstest.Repo{}
	ana := create(t, repo, "Ana", "83999991234")
	create(t, repo, "Bia", "83988887766")

	_, err := commands.UpdateHandler{Repository: repo}.Handle(context.Background(), ports.UpdateSeller{UID: ana, Name: "Ana", WhatsApp: "83988887766", Active: true})
	var conflict httperr.ConflictError
	assert.True(t, errors.As(err, &conflict), "número da Bia")

	_, err = commands.UpdateHandler{Repository: repo}.Handle(context.Background(), ports.UpdateSeller{UID: ana, Name: "Ana Maria", WhatsApp: "83999991234", Active: true})
	assert.NoError(t, err, "salvar com o próprio número não é duplicidade")
}

func TestUpdateAndDelete_UnknownSellerIsNotFound(t *testing.T) {
	repo := &sellerstest.Repo{}
	var nf httperr.NotFoundError

	_, err := commands.UpdateHandler{Repository: repo}.Handle(context.Background(), ports.UpdateSeller{UID: "nao-existe", Name: "Ana", WhatsApp: "83999991234", Active: true})
	assert.True(t, errors.As(err, &nf))
	err = commands.DeleteHandler{Repository: repo}.Handle(context.Background(), "nao-existe")
	assert.True(t, errors.As(err, &nf))
}

func TestDelete_RemovesTheSeller(t *testing.T) {
	repo := &sellerstest.Repo{}
	uid := create(t, repo, "Ana", "83999991234")

	require.NoError(t, commands.DeleteHandler{Repository: repo}.Handle(context.Background(), uid))

	assert.Empty(t, repo.Sellers)
}
