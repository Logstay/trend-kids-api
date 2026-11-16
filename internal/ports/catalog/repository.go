package ports

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
)

type Sort string

const (
	SortNewest    Sort = "newest"
	SortPriceAsc  Sort = "price_asc"
	SortPriceDesc Sort = "price_desc"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
	// MaxImages: fotos por produto.
	MaxImages = 8
)

type Filter struct {
	Category string
	Query    string
	Sort     Sort
	Limit    int
	Offset   int
	// IncludeInactive é só para o painel; a vitrine pública nunca enxerga produto inativo.
	IncludeInactive bool
}

// Repository é o contrato de persistência do catálogo.
// Get devolve httperr.NotFoundError quando o produto não existe.
type Repository interface {
	Save(ctx context.Context, product catalog.Product) error
	GetBySlug(ctx context.Context, slug string) (catalog.Product, error)
	ExistsBySlug(ctx context.Context, slug string) (bool, error)
	List(ctx context.Context, filter Filter) ([]catalog.Product, error)

	// GetByUID enxerga produtos inativos; devolve httperr.NotFoundError se não existir.
	GetByUID(ctx context.Context, uid string) (catalog.Product, error)
	// Update grava nome, categoria, descrição, preço e ativo. Nunca altera o estoque.
	Update(ctx context.Context, product catalog.Product) error
	// AdjustStock soma delta ao estoque de forma atômica e registra a movimentação.
	// Devolve o novo estoque; httperr.ConflictError se o saldo ficaria negativo.
	AdjustStock(ctx context.Context, m StockMovement) (newStock int, err error)
	// AddImage põe a foto no FIM da galeria (a primeira é a capa). httperr.ConflictError se a galeria já tem
	// MaxImages fotos (garantido de forma atômica, mesmo com envios simultâneos) ou se a chave já existe.
	AddImage(ctx context.Context, uid, key string) error
	// RemoveImage tira a foto e mantém a ordem das demais. httperr.NotFoundError se a foto não é deste produto.
	RemoveImage(ctx context.Context, uid, key string) error
	// ReorderImages aplica a nova ordem. Precisa ser EXATAMENTE as fotos atuais, em qualquer ordem
	// (httperr.RequiredError caso contrário). A primeira passa a ser a capa.
	ReorderImages(ctx context.Context, uid string, keys []string) error
}

type StockMovement struct {
	ProductUID string
	Delta      int
	Reason     string
	AdminUID   string
}
