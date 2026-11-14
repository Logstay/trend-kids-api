// Package local guarda as fotos em uma pasta do disco.
package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Logstay/trend-kids-api/internal/domain/media"
)

type Storage struct{ dir string }

// New garante que a pasta exista. Em produção ela precisa ser um volume persistente.
func New(dir string) (*Storage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("pasta de uploads: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("pasta de uploads inválida: %s", dir)
	}
	return &Storage{dir: dir}, nil
}

var errBadKey = errors.New("chave de arquivo inválida")

// Save escreve em um arquivo temporário e só então renomeia: quem lê nunca vê um arquivo pela metade.
func (s *Storage) Save(_ context.Context, key string, data []byte) error {
	if !media.ValidKey(key) {
		return errBadKey
	}
	tmp, err := os.CreateTemp(s.dir, ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(s.dir, key)); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (s *Storage) Delete(_ context.Context, key string) error {
	if !media.ValidKey(key) {
		return errBadKey
	}
	if err := os.Remove(filepath.Join(s.dir, key)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Storage) URL(key string) string {
	if key == "" {
		return ""
	}
	return "/uploads/" + key
}

// Dir é a pasta raiz, usada pelo servidor HTTP para servir os arquivos.
func (s *Storage) Dir() string { return s.dir }
