package http

import (
	nethttp "net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Deps struct {
	List           ListUseCase
	Get            GetUseCase
	AllowedOrigins []string
}

func NewRouter(d Deps) nethttp.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins: d.AllowedOrigins,
		AllowMethods: []string{nethttp.MethodGet, nethttp.MethodOptions},
		AllowHeaders: []string{"Content-Type"},
	}))

	r.GET("/health", func(c *gin.Context) { c.JSON(nethttp.StatusOK, gin.H{"status": "ok"}) })

	h := catalogHandler{list: d.List, get: d.Get}
	v1 := r.Group("/v1")
	v1.GET("/products", h.List)
	v1.GET("/products/:slug", h.Get)

	return r
}
