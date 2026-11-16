package catalog

import "time"

type Category string

const (
	CategoryClassic   Category = "Classic"
	CategoryComLuz    Category = "Com luz"
	CategorySandalias Category = "Sandálias"
	CategoryForrados  Category = "Forrados"
	CategoryCharms    Category = "Charms"
)

var validCategories = map[Category]struct{}{
	CategoryClassic: {}, CategoryComLuz: {}, CategorySandalias: {}, CategoryForrados: {}, CategoryCharms: {},
}

// Product é um item do catálogo. Dinheiro sempre em centavos.
type Product interface {
	UID() string
	Name() string
	Slug() string
	Category() Category
	Description() string
	PriceCents() int64
	Stock() int
	Active() bool
	// Images são as chaves dos arquivos de foto, em ordem de exibição (a primeira é a capa).
	// A URL de cada uma é montada pelo armazenamento. Devolve uma cópia.
	Images() []string
	// Image é a capa (a primeira foto); vazio se o produto não tem foto.
	Image() string
	CreatedAt() time.Time
	UpdatedAt() time.Time
}

type product struct {
	uid         string
	name        string
	slug        string
	category    Category
	description string
	priceCents  int64
	stock       int
	active      bool
	images      []string
	createdAt   time.Time
	updatedAt   time.Time
}

func (p *product) UID() string          { return p.uid }
func (p *product) Name() string         { return p.name }
func (p *product) Slug() string         { return p.slug }
func (p *product) Category() Category   { return p.category }
func (p *product) Description() string  { return p.description }
func (p *product) PriceCents() int64    { return p.priceCents }
func (p *product) Stock() int           { return p.stock }
func (p *product) Active() bool         { return p.active }
func (p *product) CreatedAt() time.Time { return p.createdAt }
func (p *product) UpdatedAt() time.Time { return p.updatedAt }

func (p *product) Images() []string { return append([]string(nil), p.images...) }

func (p *product) Image() string {
	if len(p.images) == 0 {
		return ""
	}
	return p.images[0]
}
