package referti

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKindLocalAndFSE(t *testing.T) {
	t.Run("local kind", func(t *testing.T) {
		got, err := ParseKind("local")
		require.NoError(t, err)
		assert.Equal(t, KindLocal, got)
	})

	t.Run("fse rejected", func(t *testing.T) {
		_, err := ParseKind("fse")
		require.ErrorIs(t, err, ErrUnknownKind)
	})

}

func TestKindLabelLocalAndFSE(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		assert.Equal(t, "Caricato", KindLocal.Label())
	})

	t.Run("fse", func(t *testing.T) {
		assert.Equal(t, "Fascicolo Sanitario", KindFSE.Label())
	})
}

func TestRefKey(t *testing.T) {
	t.Run("remote ref round trip", func(t *testing.T) {
		ref := Ref{Kind: KindLab, AbstractID: "a1", DetailID: "d1", RepCode: "HDIS-RT:REP-900"}
		got, err := ParseRefKey(ref.Key())
		require.NoError(t, err)
		assert.Equal(t, ref, got)
	})

	t.Run("local ref round trip", func(t *testing.T) {
		ref := Ref{Kind: KindLocal, AbstractID: "abc"}
		got, err := ParseRefKey(ref.Key())
		require.NoError(t, err)
		assert.Equal(t, ref, got)
	})
}

func TestParseRefKey(t *testing.T) {
	t.Run("wrong part count", func(t *testing.T) {
		_, err := ParseRefKey("lab\x1fa1")
		require.ErrorIs(t, err, ErrUnknownKind)
	})

	t.Run("unknown kind", func(t *testing.T) {
		_, err := ParseRefKey("nope\x1fa\x1fb\x1fc")
		require.ErrorIs(t, err, ErrUnknownKind)
	})
}
