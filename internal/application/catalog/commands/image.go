package commands

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	"github.com/Logstay/trend-kids-api/internal/domain/media"
	catalogports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	mediaports "github.com/Logstay/trend-kids-api/internal/ports/media"
)

type SetImageHandler struct {
	Products catalogports.Repository
	Storage  mediaports.Storage
}

// Handle valida o arquivo pelo conteúdo, grava com um nome gerado aqui e só então liga ao produto.
// Em qualquer falha não deixa arquivo órfão: se o banco falha, o arquivo recém-gravado é desfeito.
func (h SetImageHandler) Handle(ctx context.Context, uid string, data []byte) (catalog.Product, error) {
	info, err := media.Detect(data)
	if err != nil {
		return nil, err
	}
	// Confere o produto ANTES de gravar qualquer coisa em disco.
	if _, err := h.Products.GetByUID(ctx, uid); err != nil {
		return nil, err
	}

	key := media.NewKey(info.Ext)
	if err := h.Storage.Save(ctx, key, data); err != nil {
		return nil, err
	}

	previous, err := h.Products.SetImage(ctx, uid, key)
	if err != nil {
		_ = h.Storage.Delete(ctx, key) // desfaz: o arquivo novo não está ligado a nada
		return nil, err
	}
	if previous != "" {
		// A foto nova já está no ar. Se apagar a antiga falhar, sobra um arquivo velho no disco,
		// mas isso não é um erro para quem enviou a foto.
		_ = h.Storage.Delete(ctx, previous)
	}
	return h.Products.GetByUID(ctx, uid)
}

type RemoveImageHandler struct {
	Products catalogports.Repository
	Storage  mediaports.Storage
}

func (h RemoveImageHandler) Handle(ctx context.Context, uid string) (catalog.Product, error) {
	previous, err := h.Products.SetImage(ctx, uid, "")
	if err != nil {
		return nil, err
	}
	if previous != "" {
		_ = h.Storage.Delete(ctx, previous)
	}
	return h.Products.GetByUID(ctx, uid)
}
