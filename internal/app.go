package internal

import (
	nethttp "net/http"

	apphttp "github.com/Logstay/trend-kids-api/internal/adapters/http"
	pgcatalog "github.com/Logstay/trend-kids-api/internal/adapters/postgres/catalog"
	"github.com/Logstay/trend-kids-api/internal/application/catalog/queries"
	"github.com/jmoiron/sqlx"
)

// NewHandler liga repositórios, casos de uso e HTTP. É o único lugar que conhece todas as camadas.
func NewHandler(db *sqlx.DB, allowedOrigins []string) nethttp.Handler {
	catalogRepo := pgcatalog.NewRepository(db)

	return apphttp.NewRouter(apphttp.Deps{
		List:           queries.ListHandler{Repository: catalogRepo},
		Get:            queries.GetHandler{Repository: catalogRepo},
		AllowedOrigins: allowedOrigins,
	})
}
