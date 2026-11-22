package textutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFold(t *testing.T) {
	t.Run("lowercases", func(t *testing.T) {
		assert.Equal(t, "ospedale", Fold("OSPEDALE"))
	})

	t.Run("strips accents", func(t *testing.T) {
		assert.Equal(t, "citta di firenze", Fold("Città di Firenze"))
	})

	t.Run("collapses and trims whitespace", func(t *testing.T) {
		assert.Equal(t, "a b c", Fold("  a \t b\n c  "))
	})

	t.Run("empty string", func(t *testing.T) {
		assert.Equal(t, "", Fold(""))
	})
}
