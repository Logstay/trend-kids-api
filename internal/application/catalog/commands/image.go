package commands

import (
	"context"
	"fmt"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	"github.com/Logstay/trend-kids-api/internal/domain/media"
	catalogports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	mediaports "github.com/Logstay/trend-kids-api/internal/ports/media"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type AddImageHandler struct {
	Products catalogports.Repository
	Storage  mediaports.Storage
}

// Handle valida o arquivo pelo conteúdo, grava com um nome gerado aqui e só então põe na galeria do produto.
// Em qualquer falha não deixa arquivo órfão: se o banco falha, o arquivo recém-gravado é desfeito.
func (h AddImageHandler) Handle(ctx context.Context, uid string, data []byte) (catalog.Product, error) {
	info, err := media.Detect(data)
	if err != nil {
		return nil, err
	}
	// Confere o produto e o limite ANTES de gravar qualquer coisa em disco.
	product, err := h.Products.GetByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if len(product.Images()) >= catalogports.MaxImages {
		return nil, httperr.ConflictError(fmt.Sprintf("limite de %d fotos por produto atingido", catalogports.MaxImages))
	}

	key := media.NewKey(info.Ext)
	if err := h.Storage.Save(ctx, key, data); err != nil {
		return nil, err
	}
	// O limite é garantido de novo, de forma atômica, pelo repositório (envios simultâneos).
	if err := h.Products.AddImage(ctx, uid, key); err != nil {
		_ = h.Storage.Delete(ctx, key) // desfaz: o arquivo novo não está ligado a nada
		return nil, err
	}
	return h.Products.GetByUID(ctx, uid)
}

type RemoveImageHandler struct {
	Products catalogports.Repository
	Storage  mediaports.Storage
}

// Handle tira a foto da galeria e apaga o arquivo. A chave vem do cliente, então só uma chave no formato
// exato que o servidor gera chega perto do repositório e do disco.
func (h RemoveImageHandler) Handle(ctx context.Context, uid, key string) (catalog.Product, error) {
	if !media.ValidKey(key) {
		return nil, httperr.NotFoundError("foto não encontrada")
	}
	if err := h.Products.RemoveImage(ctx, uid, key); err != nil {
		return nil, err
	}
	// A foto já saiu da galeria. Se apagar o arquivo falhar, sobra um arquivo no disco, mas isso
	// não é um erro para quem pediu a remoção.
	_ = h.Storage.Delete(ctx, key)
	return h.Products.GetByUID(ctx, uid)
}

type ReorderImagesHandler struct {
	Products catalogports.Repository
}

// Handle aplica a nova ordem. A primeira passa a ser a capa.
func (h ReorderImagesHandler) Handle(ctx context.Context, uid string, keys []string) (catalog.Product, error) {
	for _, k := range keys {
		if !media.ValidKey(k) {
			return nil, httperr.RequiredError("a nova ordem tem uma foto inválida")
		}
	}
	if err := h.Products.ReorderImages(ctx, uid, keys); err != nil {
		return nil, err
	}
	return h.Products.GetByUID(ctx, uid)
}
