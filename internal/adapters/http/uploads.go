package http

import (
	nethttp "net/http"
	"os"
	"path/filepath"

	"github.com/Logstay/trend-kids-api/internal/domain/media"
	"github.com/gin-gonic/gin"
)

var contentTypes = map[string]string{".jpg": "image/jpeg", ".png": "image/png", ".webp": "image/webp"}

// serveUpload serve as fotos. Só chaves no formato exato que o servidor gera (uuid + .jpg/.png/.webp) são aceitas:
// "../", arquivos temporários, listagem de pasta e qualquer outro nome dão 404, sem tocar no disco.
func serveUpload(dir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		if !media.ValidKey(key) {
			c.Status(nethttp.StatusNotFound)
			return
		}
		path := filepath.Join(dir, key)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			c.Status(nethttp.StatusNotFound) // 404 sem cache: a foto pode ainda ser gravada
			return
		}

		h := c.Writer.Header()
		h.Set("Content-Type", contentTypes[filepath.Ext(key)])
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "default-src 'none'; sandbox") // mesmo que algo estranho passasse, não executa nada
		h.Set("Cache-Control", "public, max-age=31536000, immutable")   // o nome é único: trocar a foto muda a URL
		c.File(path)
	}
}
