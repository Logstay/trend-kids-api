package internal

import (
	"log/slog"
	nethttp "net/http"
	"time"

	"github.com/Logstay/trend-kids-api/config"
	apphttp "github.com/Logstay/trend-kids-api/internal/adapters/http"
	pgcatalog "github.com/Logstay/trend-kids-api/internal/adapters/postgres/catalog"
	pgidentity "github.com/Logstay/trend-kids-api/internal/adapters/postgres/identity"
	pgorders "github.com/Logstay/trend-kids-api/internal/adapters/postgres/orders"
	pgsettings "github.com/Logstay/trend-kids-api/internal/adapters/postgres/settings"
	"github.com/Logstay/trend-kids-api/internal/adapters/security"
	catalogcommands "github.com/Logstay/trend-kids-api/internal/application/catalog/commands"
	catalogqueries "github.com/Logstay/trend-kids-api/internal/application/catalog/queries"
	identitycommands "github.com/Logstay/trend-kids-api/internal/application/identity/commands"
	ordercommands "github.com/Logstay/trend-kids-api/internal/application/orders/commands"
	orderqueries "github.com/Logstay/trend-kids-api/internal/application/orders/queries"
	settingscommands "github.com/Logstay/trend-kids-api/internal/application/settings/commands"
	settingsqueries "github.com/Logstay/trend-kids-api/internal/application/settings/queries"
	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	"github.com/jmoiron/sqlx"
)

// NewHandler liga repositórios, casos de uso e HTTP. É o único lugar que conhece todas as camadas.
func NewHandler(db *sqlx.DB, cfg *config.Config, logger *slog.Logger) (nethttp.Handler, error) {
	tokens, err := security.NewTokenService(cfg.JWTSecret, cfg.SessionTTL, time.Now)
	if err != nil {
		return nil, err
	}

	catalogRepo := pgcatalog.NewRepository(db)
	adminRepo := pgidentity.NewRepository(db)
	orderRepo := pgorders.NewRepository(db)
	settingsRepo := pgsettings.NewRepository(db)
	shipping := orders.ShippingPolicy{FlatCents: cfg.ShippingFlatCents, FreeAboveCents: cfg.ShippingFreeAboveCents}

	return apphttp.NewRouter(apphttp.Deps{
		List: catalogqueries.ListHandler{Repository: catalogRepo},
		Catalog: &apphttp.AdminCatalog{
			List:        catalogqueries.ListHandler{Repository: catalogRepo},
			Create:      catalogcommands.SaveHandler{Repository: catalogRepo},
			Update:      catalogcommands.UpdateHandler{Repository: catalogRepo},
			AdjustStock: catalogcommands.AdjustStockHandler{Repository: catalogRepo},
		},
		Get:            catalogqueries.GetHandler{Repository: catalogRepo},
		AllowedOrigins: cfg.AllowedOrigins,
		Logger:         logger,
		TrustedProxies: cfg.TrustedProxies,
		Settings: &apphttp.SettingsRoutes{
			Get:    settingsqueries.GetHandler{Repository: settingsRepo},
			Update: settingscommands.UpdateHandler{Repository: settingsRepo},
		},
		AdminOrders: &apphttp.AdminOrders{
			List:         orderqueries.AdminListHandler{Orders: orderRepo},
			Get:          orderqueries.AdminGetHandler{Orders: orderRepo},
			ChangeStatus: ordercommands.ChangeStatusHandler{Orders: orderRepo},
		},
		Orders: &apphttp.Orders{
			Place:  ordercommands.PlaceHandler{Orders: orderRepo, Products: catalogRepo, Shipping: shipping},
			Lookup: orderqueries.LookupHandler{Orders: orderRepo},
		},
		Login:        identitycommands.LoginHandler{Repository: adminRepo, Hasher: security.NewArgon2Hasher(), Tokens: tokens},
		Tokens:       tokens,
		CookieSecure: cfg.CookieSecure,
	}), nil
}

// NewCreateAdmin devolve o caso de uso usado pelo comando cmd/createadmin.
func NewCreateAdmin(db *sqlx.DB) identitycommands.CreateAdminHandler {
	return identitycommands.CreateAdminHandler{Repository: pgidentity.NewRepository(db), Hasher: security.NewArgon2Hasher()}
}
