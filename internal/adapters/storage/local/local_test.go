package local_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/adapters/storage/local"
	"github.com/Logstay/trend-kids-api/internal/domain/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStorage(t *testing.T) (*local.Storage, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "uploads")
	s, err := local.New(dir)
	require.NoError(t, err)
	return s, dir
}

func TestNew_CreatesTheDirectoryAndRefusesAFile(t *testing.T) {
	_, dir := newStorage(t)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	file := filepath.Join(t.TempDir(), "arquivo")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	_, err = local.New(file)
	assert.Error(t, err)
}

func TestSave_WritesTheBytesAndLeavesNoTemporaryFiles(t *testing.T) {
	s, dir := newStorage(t)
	key := media.NewKey(".png")

	require.NoError(t, s.Save(context.Background(), key, []byte("conteudo")))

	got, err := os.ReadFile(filepath.Join(dir, key))
	require.NoError(t, err)
	assert.Equal(t, "conteudo", string(got))
	entries, _ := os.ReadDir(dir)
	require.Len(t, entries, 1, "só o arquivo final, sem temporários")
	assert.Equal(t, key, entries[0].Name())
}

func TestSaveAndDelete_RefuseKeysThatCouldEscapeTheDirectory(t *testing.T) {
	s, dir := newStorage(t)
	outside := filepath.Join(filepath.Dir(dir), "fora.jpg")
	require.NoError(t, os.WriteFile(outside, []byte("nao-apague"), 0o600))

	for _, bad := range []string{"../fora.jpg", "/etc/passwd", "a/b.png", "", "index.html", "..\\fora.jpg"} {
		assert.Error(t, s.Save(context.Background(), bad, []byte("x")), "Save %q", bad)
		assert.Error(t, s.Delete(context.Background(), bad), "Delete %q", bad)
	}
	still, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "nao-apague", string(still), "nada fora da pasta foi tocado")
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries)
}

func TestDelete_RemovesAndIsIdempotent(t *testing.T) {
	s, dir := newStorage(t)
	key := media.NewKey(".jpg")
	require.NoError(t, s.Save(context.Background(), key, []byte("x")))

	require.NoError(t, s.Delete(context.Background(), key))
	_, err := os.Stat(filepath.Join(dir, key))
	assert.True(t, os.IsNotExist(err))
	assert.NoError(t, s.Delete(context.Background(), key), "apagar o que já não existe não é erro")
}

func TestURL(t *testing.T) {
	s, _ := newStorage(t)
	key := media.NewKey(".webp")
	assert.Equal(t, "/uploads/"+key, s.URL(key))
	assert.Equal(t, "", s.URL(""), "produto sem foto não tem URL")
}

func TestSave_ConcurrentWritesAllLand(t *testing.T) {
	s, dir := newStorage(t)
	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, s.Save(context.Background(), media.NewKey(".png"), []byte("x")))
		}()
	}
	wg.Wait()
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, n)
}

func TestDir_ExposesTheRootForServing(t *testing.T) {
	s, dir := newStorage(t)
	assert.Equal(t, dir, s.Dir())
}
