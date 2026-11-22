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

func TestCompareNatural(t *testing.T) {
	t.Run("orders numbers numerically", func(t *testing.T) {
		assert.Negative(t, CompareNatural("Visita 2", "Visita 10"))
	})

	t.Run("ignores case", func(t *testing.T) {
		assert.Zero(t, CompareNatural("ospedale", "OSPEDALE"))
	})

	t.Run("ignores accents", func(t *testing.T) {
		assert.Zero(t, CompareNatural("citta", "Città"))
	})

	t.Run("orders letters alphabetically", func(t *testing.T) {
		assert.Negative(t, CompareNatural("Arezzo", "Firenze"))
	})

	t.Run("greater sorts after", func(t *testing.T) {
		assert.Positive(t, CompareNatural("Firenze", "Arezzo"))
	})
}

func TestContainsFolded(t *testing.T) {
	t.Run("matches ignoring case and accents", func(t *testing.T) {
		assert.True(t, ContainsFolded("Ospedale di Città", "CITTA"))
	})

	t.Run("no match", func(t *testing.T) {
		assert.False(t, ContainsFolded("Ospedale", "radiologia"))
	})

	t.Run("empty needle matches", func(t *testing.T) {
		assert.True(t, ContainsFolded("Ospedale", "  "))
	})
}
