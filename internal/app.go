package internal

import (
	nethttp "net/http"
	"time"

	"github.com/Logstay/trend-kids-api/config"
	apphttp "github.com/Logstay/trend-kids-api/internal/adapters/http"
	pgcatalog "github.com/Logstay/trend-kids-api/internal/adapters/postgres/catalog"
	pgidentity "github.com/Logstay/trend-kids-api/internal/adapters/postgres/identity"
	"github.com/Logstay/trend-kids-api/internal/adapters/security"
	catalogqueries "github.com/Logstay/trend-kids-api/internal/application/catalog/queries"
	identitycommands "github.com/Logstay/trend-kids-api/internal/application/identity/commands"
	"github.com/jmoiron/sqlx"
)

// NewHandler liga repositórios, casos de uso e HTTP. É o único lugar que conhece todas as camadas.
func NewHandler(db *sqlx.DB, cfg *config.Config) (nethttp.Handler, error) {
	tokens, err := security.NewTokenService(cfg.JWTSecret, cfg.SessionTTL, time.Now)
	if err != nil {
		return nil, err
	}

	catalogRepo := pgcatalog.NewRepository(db)
	adminRepo := pgidentity.NewRepository(db)

	return apphttp.NewRouter(apphttp.Deps{
		List:           catalogqueries.ListHandler{Repository: catalogRepo},
		Get:            catalogqueries.GetHandler{Repository: catalogRepo},
		AllowedOrigins: cfg.AllowedOrigins,
		Login:          identitycommands.LoginHandler{Repository: adminRepo, Hasher: security.NewArgon2Hasher(), Tokens: tokens},
		Tokens:         tokens,
		CookieSecure:   cfg.CookieSecure,
	}), nil
}

// NewCreateAdmin devolve o caso de uso usado pelo comando cmd/createadmin.
func NewCreateAdmin(db *sqlx.DB) identitycommands.CreateAdminHandler {
	return identitycommands.CreateAdminHandler{Repository: pgidentity.NewRepository(db), Hasher: security.NewArgon2Hasher()}
}
