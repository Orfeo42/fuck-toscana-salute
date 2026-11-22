package listview

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPageSizeOrDefault(t *testing.T) {
	t.Run("zero falls back to the default", func(t *testing.T) {
		assert.Equal(t, DefaultPageSize, PageSizeOrDefault(0))
	})

	t.Run("negative falls back to the default", func(t *testing.T) {
		assert.Equal(t, DefaultPageSize, PageSizeOrDefault(-5))
	})

	t.Run("a positive size is kept", func(t *testing.T) {
		assert.Equal(t, 25, PageSizeOrDefault(25))
	})
}

func TestPaginate(t *testing.T) {
	t.Run("first page", func(t *testing.T) {
		assert.Equal(t, Window{Page: 1, PageSize: 25, Start: 0, End: 25}, Paginate(60, 1, 25))
	})

	t.Run("last partial page", func(t *testing.T) {
		assert.Equal(t, Window{Page: 3, PageSize: 25, Start: 50, End: 60}, Paginate(60, 3, 25))
	})

	t.Run("a page past the end clamps to the last page", func(t *testing.T) {
		assert.Equal(t, Window{Page: 3, PageSize: 25, Start: 50, End: 60}, Paginate(60, 99, 25))
	})

	t.Run("a page below one clamps to the first page", func(t *testing.T) {
		assert.Equal(t, Window{Page: 1, PageSize: 25, Start: 0, End: 25}, Paginate(60, -2, 25))
	})

	t.Run("no rows gives page one and an empty window", func(t *testing.T) {
		assert.Equal(t, Window{Page: 1, PageSize: 50, Start: 0, End: 0}, Paginate(0, 4, 0))
	})
}

func TestDirected(t *testing.T) {
	t.Run("ascending keeps the sign", func(t *testing.T) {
		assert.Equal(t, 1, Directed(1, DirAsc))
	})

	t.Run("descending flips the sign", func(t *testing.T) {
		assert.Equal(t, -1, Directed(1, DirDesc))
	})
}

func TestMissingLast(t *testing.T) {
	t.Run("both missing are equal", func(t *testing.T) {
		assert.Equal(t, 0, MissingLast(true, true))
	})

	t.Run("a missing first sorts after", func(t *testing.T) {
		assert.Equal(t, 1, MissingLast(true, false))
	})

	t.Run("a missing second sorts before", func(t *testing.T) {
		assert.Equal(t, -1, MissingLast(false, true))
	})
}
