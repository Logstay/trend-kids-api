package queries_test

import (
	"context"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/settings/queries"
	"github.com/Logstay/trend-kids-api/internal/ports/settings/settingstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet_ReturnsCurrentSettings(t *testing.T) {
	got, err := queries.GetHandler{Repository: settingstest.New()}.Handle(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Trend Kids JP", got.Name())
}
