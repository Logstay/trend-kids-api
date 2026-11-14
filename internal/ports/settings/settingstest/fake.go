// Package settingstest traz um repositório de configurações em memória para testes.
package settingstest

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/settings"
)

type Repo struct {
	Current   settings.Settings
	UpdatedBy []string
	Updates   int
}

func New() *Repo {
	return &Repo{Current: settings.Restore(settings.Snapshot{Name: "Trend Kids JP", City: "João Pessoa · PB"})}
}

func (r *Repo) Get(context.Context) (settings.Settings, error) { return r.Current, nil }

func (r *Repo) Update(_ context.Context, s settings.Settings, adminUID string) error {
	r.Current = s
	r.Updates++
	r.UpdatedBy = append(r.UpdatedBy, adminUID)
	return nil
}
