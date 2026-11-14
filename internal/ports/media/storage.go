package ports

import "context"

// Storage guarda os arquivos de foto. Hoje há um adaptador de disco local; em produção dá para trocar por
// S3/R2 sem mexer nos casos de uso.
type Storage interface {
	// Save grava os bytes sob a chave (que precisa ser válida: ver media.ValidKey).
	Save(ctx context.Context, key string, data []byte) error
	// Delete apaga o arquivo. Apagar o que já não existe não é erro.
	Delete(ctx context.Context, key string) error
	// URL devolve o endereço público da foto; vazio se a chave for vazia.
	URL(key string) string
}
