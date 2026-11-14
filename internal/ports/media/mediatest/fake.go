// Package mediatest traz um armazenamento em memória para testes dos casos de uso.
package mediatest

import (
	"context"
	"errors"
)

type Storage struct {
	Files     map[string][]byte
	Deleted   []string
	SaveErr   error
	DeleteErr error
}

func New() *Storage { return &Storage{Files: map[string][]byte{}} }

func (s *Storage) Save(_ context.Context, key string, data []byte) error {
	if s.SaveErr != nil {
		return s.SaveErr
	}
	s.Files[key] = data
	return nil
}

func (s *Storage) Delete(_ context.Context, key string) error {
	s.Deleted = append(s.Deleted, key)
	if s.DeleteErr != nil {
		return s.DeleteErr
	}
	delete(s.Files, key)
	return nil
}

func (s *Storage) URL(key string) string {
	if key == "" {
		return ""
	}
	return "/uploads/" + key
}

var ErrDisk = errors.New("disco cheio")
