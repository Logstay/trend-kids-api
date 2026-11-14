package catalog

import (
	"regexp"
	"strings"
	"unicode"

	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

type ProductFactory interface {
	Name(name string) error
	Category(category string) error
	PriceCents(cents int64) error
	Stock(stock int) error
	Description(description string)
	Build() Product
}

type productFactory struct {
	product *product
}

func NewProductFactory() ProductFactory {
	return &productFactory{product: &product{uid: uuid.NewString(), active: true}}
}

func (f *productFactory) Name(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return httperr.RequiredError("nome do produto é obrigatório")
	}
	f.product.name = name
	f.product.slug = slugify(name)
	return nil
}

func (f *productFactory) Category(category string) error {
	c := Category(category)
	if _, ok := validCategories[c]; !ok {
		return httperr.RequiredError("categoria inválida")
	}
	f.product.category = c
	return nil
}

func (f *productFactory) PriceCents(cents int64) error {
	if cents <= 0 {
		return httperr.RequiredError("preço do produto é obrigatório")
	}
	f.product.priceCents = cents
	return nil
}

func (f *productFactory) Stock(stock int) error {
	if stock < 0 {
		return httperr.RequiredError("estoque não pode ser negativo")
	}
	f.product.stock = stock
	return nil
}

func (f *productFactory) Description(description string) {
	f.product.description = strings.TrimSpace(description)
}

func (f *productFactory) Build() Product { return f.product }

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return strings.Trim(nonSlug.ReplaceAllString(b.String(), "-"), "-")
}
