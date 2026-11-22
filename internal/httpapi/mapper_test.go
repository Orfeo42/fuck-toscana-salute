package httpapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/session"
)

func TestToSessionBody(t *testing.T) {
	t.Run("zero times map to nil pointers", func(t *testing.T) {
		status := session.Status{Portals: []session.PortalStatus{{Portal: session.PortalCUP, State: session.StateLoggedOut}}}
		got := toSessionBody(status, nil, "window", false)
		require.Len(t, got.Portals, 1)
		assert.Nil(t, got.Portals[0].AccessExpiresAt)
		assert.Nil(t, got.Portals[0].RefreshExpiresAt)
	})

	t.Run("non-zero times are kept", func(t *testing.T) {
		at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		status := session.Status{Portals: []session.PortalStatus{{Portal: session.PortalFSE, State: session.StateActive, AccessExpiresAt: at}}}
		got := toSessionBody(status, nil, "window", false)
		require.NotNil(t, got.Portals[0].AccessExpiresAt)
		assert.True(t, at.Equal(*got.Portals[0].AccessExpiresAt))
	})

	t.Run("missing login state defaults to idle", func(t *testing.T) {
		status := session.Status{Portals: []session.PortalStatus{{Portal: session.PortalCUP}}}
		got := toSessionBody(status, map[session.Portal]loginflow.State{}, "window", false)
		assert.Equal(t, LoginBody{Phase: "idle"}, got.Portals[0].Login)
	})

	t.Run("login state is copied and order preserved", func(t *testing.T) {
		status := session.Status{
			Portals:       []session.PortalStatus{{Portal: session.PortalFSE}, {Portal: session.PortalCUP}},
			GestioneReady: true,
		}
		states := map[session.Portal]loginflow.State{
			session.PortalCUP: {Phase: loginflow.PhaseFailed, Error: "boom"},
		}
		got := toSessionBody(status, states, "window", false)
		assert.True(t, got.GestioneReady)
		assert.Equal(t, "fse", got.Portals[0].Portal)
		assert.Equal(t, LoginBody{Phase: "idle"}, got.Portals[0].Login)
		assert.Equal(t, LoginBody{Phase: "failed", Error: "boom"}, got.Portals[1].Login)
	})

	t.Run("no portals gives empty non-nil slice", func(t *testing.T) {
		got := toSessionBody(session.Status{}, nil, "window", false)
		assert.NotNil(t, got.Portals)
		assert.Empty(t, got.Portals)
	})

	t.Run("carries the viewer flag", func(t *testing.T) {
		assert.True(t, toSessionBody(session.Status{}, nil, "window", true).Viewer)
		assert.False(t, toSessionBody(session.Status{}, nil, "window", false).Viewer)
	})
}

func TestToLocalReportBody(t *testing.T) {
	created := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	t.Run("maps the report with an nre", func(t *testing.T) {
		got := toLocalReportBody(localrecords.Report{ID: "abc", Meta: localrecords.Meta{Title: "Visita", Provider: "ASL"}, CreatedAt: created}, "0900A0000000001")

		assert.Equal(t, ReportBody{
			Kind:       "local",
			KindLabel:  referti.KindLocal.Label(),
			AbstractID: "abc",
			UpdatedAt:  &created,
			Provider:   "ASL",
			Origin:     "local",
			Title:      "Visita",
			NREs:       []string{"0900A0000000001"},
		}, got)
	})

	t.Run("no nre gives an empty list and no zero date", func(t *testing.T) {
		got := toLocalReportBody(localrecords.Report{ID: "abc", CreatedAt: created}, "")

		assert.Equal(t, []string{}, got.NREs)
		assert.Nil(t, got.Date)
	})
}
