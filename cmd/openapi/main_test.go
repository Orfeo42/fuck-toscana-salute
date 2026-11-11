package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("boom") }

func TestRun(t *testing.T) {
	t.Run("prints a spec with the api paths", func(t *testing.T) {
		var buf bytes.Buffer

		require.NoError(t, run(&buf))

		assert.Contains(t, buf.String(), `"/api/reports"`)
		assert.Contains(t, buf.String(), `"/api/refresh"`)
		assert.Contains(t, buf.String(), `"/api/google/settings"`)
	})

	t.Run("write failure is returned", func(t *testing.T) {
		require.Error(t, run(failingWriter{}))
	})
}
