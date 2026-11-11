package vault

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryBackend struct {
	entries map[string]string
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{entries: map[string]string{}}
}

func (m *memoryBackend) Get(service, user string) (string, error) {
	value, ok := m.entries[service+"\x00"+user]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

func (m *memoryBackend) Set(service, user, value string) error {
	m.entries[service+"\x00"+user] = value
	return nil
}

func (m *memoryBackend) Delete(service, user string) error {
	delete(m.entries, service+"\x00"+user)
	return nil
}

type failingBackend struct {
	memoryBackend
	getErr error
	setErr error
}

func (f *failingBackend) Get(service, user string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.memoryBackend.Get(service, user)
}

func (f *failingBackend) Set(service, user, value string) error {
	if f.setErr != nil {
		return f.setErr
	}
	return f.memoryBackend.Set(service, user, value)
}

func TestCopy(t *testing.T) {
	t.Run("copies missing entries", func(t *testing.T) {
		src := newMemoryBackend()
		dst := newMemoryBackend()
		require.NoError(t, src.Set(testService, "session", "a"))
		require.NoError(t, src.Set(testService, "google", "b"))

		n, err := Copy(dst, src, testService, []string{"session", "google"})

		require.NoError(t, err)
		assert.Equal(t, 2, n)
		got, err := dst.Get(testService, "google")
		require.NoError(t, err)
		assert.Equal(t, "b", got)
	})

	t.Run("keeps existing destination entry", func(t *testing.T) {
		src := newMemoryBackend()
		dst := newMemoryBackend()
		require.NoError(t, src.Set(testService, "session", "old"))
		require.NoError(t, dst.Set(testService, "session", "new"))

		n, err := Copy(dst, src, testService, []string{"session"})

		require.NoError(t, err)
		assert.Equal(t, 0, n)
		got, err := dst.Get(testService, "session")
		require.NoError(t, err)
		assert.Equal(t, "new", got)
	})

	t.Run("skips entries missing in source", func(t *testing.T) {
		n, err := Copy(newMemoryBackend(), newMemoryBackend(), testService, []string{"session"})

		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("source read error", func(t *testing.T) {
		src := &failingBackend{memoryBackend: *newMemoryBackend(), getErr: errors.New("boom")}

		_, err := Copy(newMemoryBackend(), src, testService, []string{"session"})

		require.Error(t, err)
	})

	t.Run("destination check error", func(t *testing.T) {
		src := newMemoryBackend()
		require.NoError(t, src.Set(testService, "session", "a"))
		dst := &failingBackend{memoryBackend: *newMemoryBackend(), getErr: errors.New("boom")}

		_, err := Copy(dst, src, testService, []string{"session"})

		require.Error(t, err)
	})

	t.Run("destination write error", func(t *testing.T) {
		src := newMemoryBackend()
		require.NoError(t, src.Set(testService, "session", "a"))
		dst := &failingBackend{memoryBackend: *newMemoryBackend(), setErr: errors.New("boom")}

		_, err := Copy(dst, src, testService, []string{"session"})

		require.Error(t, err)
	})
}
