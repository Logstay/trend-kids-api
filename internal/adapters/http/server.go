package http

import (
	nethttp "net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const (
	defaultLoginLimit  = 5
	defaultOrderLimit  = 10
	defaultLoginWindow = time.Minute
)

func withDefaults(limit int, window time.Duration, defaultLimit int) (int, time.Duration) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if window <= 0 {
		window = defaultLoginWindow
	}
	return limit, window
}

type Deps struct {
	List           ListUseCase
	Get            GetUseCase
	AllowedOrigins []string
	// TrustedProxies: IPs/CIDRs cujo X-Forwarded-For é aceito (o servidor Next, quando ele fica na frente).
	// Vazio = ninguém: o IP do cliente é sempre o da conexão.
	TrustedProxies []string

	// Admin: as rotas /v1/admin só existem quando Login está definido.
	Login        LoginUseCase
	Tokens       TokenParser
	CookieSecure bool
	LoginLimit   int
	LoginWindow  time.Duration

	// Catalog liga as rotas /v1/admin/products (exigem Login configurado).
	Catalog *AdminCatalog

	// AdminOrders liga as rotas /v1/admin/orders (exigem Login configurado).
	AdminOrders *AdminOrders

	// Settings liga GET /v1/settings (público) e PUT /v1/admin/settings (exige Login configurado).
	Settings *SettingsRoutes

	// Orders liga as rotas públicas /v1/orders. Limite por IP (padrão 10 por minuto, cada rota com a sua cota).
	Orders      *Orders
	OrderLimit  int
	OrderWindow time.Duration
}

func NewRouter(d Deps) nethttp.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// Por padrão ninguém é confiável: ClientIP usa o endereço real da conexão e ignora X-Forwarded-For,
	// que um atacante forjaria para escapar do limite de tentativas. Só os proxies listados valem.
	// (Se a lista for inválida o Gin mantém "ninguém": falha para o lado seguro; config.Load já valida.)
	_ = r.SetTrustedProxies(nil)
	if len(d.TrustedProxies) > 0 {
		_ = r.SetTrustedProxies(d.TrustedProxies)
	}
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     d.AllowedOrigins,
		AllowMethods:     []string{nethttp.MethodGet, nethttp.MethodPost, nethttp.MethodPut, nethttp.MethodPatch, nethttp.MethodDelete, nethttp.MethodOptions},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: true,
	}))

	r.GET("/health", func(c *gin.Context) { c.JSON(nethttp.StatusOK, gin.H{"status": "ok"}) })

	h := catalogHandler{list: d.List, get: d.Get}
	v1 := r.Group("/v1")
	v1.GET("/products", h.List)
	v1.GET("/products/:slug", h.Get)

	if d.Settings != nil {
		v1.GET("/settings", settingsHandler{SettingsRoutes: *d.Settings}.Get)
	}

	if d.Orders != nil {
		oh := ordersHandler{Orders: *d.Orders}
		limit, window := withDefaults(d.OrderLimit, d.OrderWindow, defaultOrderLimit)
		v1.POST("/orders", rateLimit(newRateLimiter(limit, window, time.Now)), oh.PlaceOrder)
		v1.POST("/orders/lookup", rateLimit(newRateLimiter(limit, window, time.Now)), oh.LookupOrder)
	}

	if d.Login != nil {
		registerAdmin(v1.Group("/admin"), d)
	}
	return r
}

func registerAdmin(g *gin.RouterGroup, d Deps) {
	limit, window := withDefaults(d.LoginLimit, d.LoginWindow, defaultLoginLimit)

	a := adminHandler{login: d.Login, tokens: d.Tokens, cookieSecure: d.CookieSecure, now: time.Now}
	g.POST("/login", rateLimit(newRateLimiter(limit, window, time.Now)), a.Login)
	g.POST("/logout", a.Logout)
	g.GET("/me", a.RequireAdmin, a.Me)

	if d.Catalog != nil {
		ch := adminCatalogHandler{AdminCatalog: *d.Catalog}
		products := g.Group("/products", a.RequireAdmin)
		products.GET("", ch.List)
		products.POST("", ch.Create)
		products.PUT("/:uid", ch.Update)
		products.PATCH("/:uid/stock", ch.AdjustStock)
	}

	if d.Settings != nil {
		g.PUT("/settings", a.RequireAdmin, settingsHandler{SettingsRoutes: *d.Settings}.Put)
	}

	if d.AdminOrders != nil {
		oh := adminOrdersHandler{AdminOrders: *d.AdminOrders}
		ord := g.Group("/orders", a.RequireAdmin)
		ord.GET("", oh.List)
		ord.GET("/:code", oh.Get)
		ord.PATCH("/:code/status", oh.ChangeStatus)
	}
}
