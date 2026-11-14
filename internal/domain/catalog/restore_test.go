package catalog_test

import (
	"testing"
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	"github.com/stretchr/testify/assert"
)

func TestRestore_RebuildsProductExactlyAsStored(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

	p := catalog.Restore(catalog.Snapshot{
		UID: "u-1", Name: "Classic Clog Rosa", Slug: "classic-clog-rosa", Category: "Classic",
		Description: "d", PriceCents: 100, Stock: 2, Active: false, CreatedAt: now, UpdatedAt: now,
	})

	assert.Equal(t, "u-1", p.UID())
	assert.Equal(t, "classic-clog-rosa", p.Slug())
	assert.Equal(t, catalog.CategoryClassic, p.Category())
	assert.False(t, p.Active(), "Restore não pode forçar active=true")
	assert.Equal(t, now, p.CreatedAt())
}
