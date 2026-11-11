package httpapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

func TestToSessionBody(t *testing.T) {
	t.Run("zero times map to nil pointers", func(t *testing.T) {
		status := session.Status{Portals: []session.PortalStatus{{Portal: session.PortalCUP, State: session.StateLoggedOut}}}
		got := toSessionBody(status, nil, "window")
		require.Len(t, got.Portals, 1)
		assert.Nil(t, got.Portals[0].AccessExpiresAt)
		assert.Nil(t, got.Portals[0].RefreshExpiresAt)
	})

	t.Run("non-zero times are kept", func(t *testing.T) {
		at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		status := session.Status{Portals: []session.PortalStatus{{Portal: session.PortalFSE, State: session.StateActive, AccessExpiresAt: at}}}
		got := toSessionBody(status, nil, "window")
		require.NotNil(t, got.Portals[0].AccessExpiresAt)
		assert.True(t, at.Equal(*got.Portals[0].AccessExpiresAt))
	})

	t.Run("missing login state defaults to idle", func(t *testing.T) {
		status := session.Status{Portals: []session.PortalStatus{{Portal: session.PortalCUP}}}
		got := toSessionBody(status, map[session.Portal]loginflow.State{}, "window")
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
		got := toSessionBody(status, states, "window")
		assert.True(t, got.GestioneReady)
		assert.Equal(t, "fse", got.Portals[0].Portal)
		assert.Equal(t, LoginBody{Phase: "idle"}, got.Portals[0].Login)
		assert.Equal(t, LoginBody{Phase: "failed", Error: "boom"}, got.Portals[1].Login)
	})

	t.Run("no portals gives empty non-nil slice", func(t *testing.T) {
		got := toSessionBody(session.Status{}, nil, "window")
		assert.NotNil(t, got.Portals)
		assert.Empty(t, got.Portals)
	})
}

func TestToPrescriptionsBodyExpiry(t *testing.T) {
	t.Run("sets expiresOn when known", func(t *testing.T) {
		at := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
		got := toPrescriptionsBody([]agenda.Row{{NRE: "0900A0000000001", FSE: &fse.Prescription{NRE: "0900A0000000001", ExpiresAt: at}}})
		require.NotNil(t, got.Prescriptions[0].FSE.ExpiresOn)
		assert.True(t, at.Equal(*got.Prescriptions[0].FSE.ExpiresOn))
	})

	t.Run("omits expiresOn when unknown", func(t *testing.T) {
		got := toPrescriptionsBody([]agenda.Row{{NRE: "0900A0000000001", FSE: &fse.Prescription{NRE: "0900A0000000001"}}})
		assert.Nil(t, got.Prescriptions[0].FSE.ExpiresOn)
	})
}

func TestToAppointmentsBodyBookingID(t *testing.T) {
	t.Run("sets bookingId", func(t *testing.T) {
		got := toAppointmentsBody(agenda.Result{Entries: []agenda.Entry{{NRE: "0900A0000000001", Appointment: cup.Appointment{ID: "1", BookingID: "ABC123456789"}}}})
		assert.Equal(t, "ABC123456789", got.Appointments[0].BookingID)
	})

	t.Run("omits bookingId from json when empty", func(t *testing.T) {
		got := toAppointmentsBody(agenda.Result{Entries: []agenda.Entry{{NRE: "0900A0000000001", Appointment: cup.Appointment{ID: "1"}}}})
		assert.Empty(t, got.Appointments[0].BookingID)
	})
}

func TestToAppointmentsBodyUnbooked(t *testing.T) {
	t.Run("copies unbooked NREs in order", func(t *testing.T) {
		got := toAppointmentsBody(agenda.Result{Unbooked: []string{"0900A0000000001", "0900A0000000002"}})
		assert.Equal(t, []string{"0900A0000000001", "0900A0000000002"}, got.Unbooked)
	})

	t.Run("nil unbooked is an empty non-nil slice", func(t *testing.T) {
		got := toAppointmentsBody(agenda.Result{})
		assert.NotNil(t, got.Unbooked)
		assert.Empty(t, got.Unbooked)
	})
}

func TestToPrescriptionsBodyExpiryEstimated(t *testing.T) {
	t.Run("sets expiryEstimated", func(t *testing.T) {
		got := toPrescriptionsBody([]agenda.Row{{NRE: "0900A0000000001", FSE: &fse.Prescription{NRE: "0900A0000000001", ExpiryEstimated: true}}})
		assert.True(t, got.Prescriptions[0].FSE.ExpiryEstimated)
	})

	t.Run("defaults to false", func(t *testing.T) {
		got := toPrescriptionsBody([]agenda.Row{{NRE: "0900A0000000001", FSE: &fse.Prescription{NRE: "0900A0000000001"}}})
		assert.False(t, got.Prescriptions[0].FSE.ExpiryEstimated)
	})
}

func TestFailureReason(t *testing.T) {
	t.Run("upstream 500 is a generic upstream failure", func(t *testing.T) {
		assert.Equal(t, "upstream_500", failureReason(&upstream.StatusError{Status: 500}))
	})
}
