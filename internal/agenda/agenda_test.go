package agenda

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/session"
)

var errBoom = errors.New("boom")

var fixedNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func newAgenda(c CUP, f FSE, concurrency int) *Agenda {
	a := New(c, f, concurrency)
	a.now = func() time.Time { return fixedNow }
	return a
}

func TestNew(t *testing.T) {
	t.Run("concurrency below one becomes one", func(t *testing.T) {
		a := New(&MockCUP{}, &MockFSE{}, 0)
		assert.Equal(t, 1, a.concurrency)
	})

	t.Run("keeps valid concurrency and sets clock", func(t *testing.T) {
		a := New(&MockCUP{}, &MockFSE{}, 4)
		assert.Equal(t, 4, a.concurrency)
		assert.NotNil(t, a.now)
	})
}

func emptyFSE() *MockFSE {
	return &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) { return nil, nil }}
}

func TestAppointments(t *testing.T) {
	t.Run("filters cancelled, done, done-after-cancel and old prescriptions", func(t *testing.T) {
		recent := fixedNow.AddDate(0, -1, 0)
		old := fixedNow.AddDate(-2, 0, 0)
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
				return []cup.Prescription{
					{NRE: "OK1", Status: cup.StatusPrescribed, PrescribedOn: recent},
					{NRE: "OK2", Status: cup.StatusBooked},
					{NRE: "CANC", Status: cup.StatusCancelled, PrescribedOn: recent},
					{NRE: "DONE", Status: cup.StatusDone, PrescribedOn: recent},
					{NRE: "DONE9", Status: cup.StatusDoneAfterCancel, PrescribedOn: recent},
					{NRE: "OLD", Status: cup.StatusPrescribed, PrescribedOn: old},
				}, nil
			},
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
				return nil, nil
			},
		}
		_, err := newAgenda(c, emptyFSE(), 2).Appointments(context.Background())
		require.NoError(t, err)
		got := []string{}
		for _, call := range c.AppointmentsCalls() {
			got = append(got, call.Nre)
		}
		assert.ElementsMatch(t, []string{"OK1", "OK2"}, got)
	})

	t.Run("sorts booked first then date ascending with zero last", func(t *testing.T) {
		d1 := fixedNow.AddDate(0, 0, 1)
		d2 := fixedNow.AddDate(0, 0, 2)
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
				return []cup.Prescription{{NRE: "A", Status: cup.StatusBooked}}, nil
			},
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
				return []cup.Appointment{
					{ID: "cancelled-early", Status: "DISDETTO", At: d1},
					{ID: "booked-zero", Status: "PRENOTATO"},
					{ID: "booked-late", Status: "PRENOTATO", At: d2},
					{ID: "booked-early", Status: "PRENOTATO", At: d1},
					{ID: "cancelled-zero", Status: "DISDETTO"},
				}, nil
			},
		}
		result, err := newAgenda(c, emptyFSE(), 1).Appointments(context.Background())
		require.NoError(t, err)
		ids := []string{}
		for _, e := range result.Entries {
			ids = append(ids, e.Appointment.ID)
			assert.Equal(t, "A", e.NRE)
		}
		assert.Equal(t, []string{"booked-early", "booked-late", "booked-zero", "cancelled-early", "cancelled-zero"}, ids)
	})

	t.Run("one failing lookup is recorded and others succeed", func(t *testing.T) {
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
				return []cup.Prescription{{NRE: "A"}, {NRE: "B"}, {NRE: "C"}}, nil
			},
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
				if nre == "B" {
					return nil, errBoom
				}
				return []cup.Appointment{{ID: "id-" + nre, Status: "PRENOTATO"}}, nil
			},
		}
		result, err := newAgenda(c, emptyFSE(), 1).Appointments(context.Background())
		require.NoError(t, err)
		require.Len(t, result.Entries, 2)
		assert.Equal(t, "id-A", result.Entries[0].Appointment.ID)
		assert.Equal(t, "id-C", result.Entries[1].Appointment.ID)
		require.Len(t, result.Failed, 1)
		assert.Equal(t, "B", result.Failed[0].NRE)
		require.ErrorIs(t, result.Failed[0].Err, errBoom)
		assert.Len(t, c.AppointmentsCalls(), 3)
	})

	t.Run("all lookups failing returns failures and no error", func(t *testing.T) {
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
				return []cup.Prescription{{NRE: "A"}, {NRE: "B"}}, nil
			},
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
				return nil, errBoom
			},
		}
		result, err := newAgenda(c, emptyFSE(), 2).Appointments(context.Background())
		require.NoError(t, err)
		assert.Empty(t, result.Entries)
		require.Len(t, result.Failed, 2)
		assert.Equal(t, "A", result.Failed[0].NRE)
		assert.Equal(t, "B", result.Failed[1].NRE)
	})

	t.Run("cancelled context fails the call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
				return []cup.Prescription{{NRE: "A"}}, nil
			},
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
				cancel()
				return nil, ctx.Err()
			},
		}
		_, err := newAgenda(c, emptyFSE(), 1).Appointments(ctx)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("fse-only visit candidate is looked up", func(t *testing.T) {
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return nil, nil },
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
				return []cup.Appointment{{ID: "id-" + nre, Status: "PRENOTATO"}}, nil
			},
		}
		f := &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) {
			return []fse.Prescription{
				{ID: "f1", NRE: "F1", Kind: fse.KindVisit, Status: 30, Date: fixedNow.AddDate(0, -1, 0)},
				{ID: "f2", NRE: "F2", Kind: fse.KindDrug, Status: 3},
				{ID: "f3", NRE: "", Kind: fse.KindVisit, Status: 3},
				{ID: "f4", NRE: "F4", Kind: fse.KindVisit, Status: 8},
				{ID: "f5", NRE: "F5", Kind: fse.KindVisit, Status: 3, Date: fixedNow.AddDate(-2, 0, 0)},
			}, nil
		}}
		result, err := newAgenda(c, f, 1).Appointments(context.Background())
		require.NoError(t, err)
		require.Len(t, result.Entries, 1)
		assert.Equal(t, "F1", result.Entries[0].NRE)
		assert.Nil(t, result.Entries[0].CUP)
		require.NotNil(t, result.Entries[0].FSE)
		assert.Equal(t, "f1", result.Entries[0].FSE.ID)
	})

	t.Run("duplicate NRE across cup and fse is looked up once", func(t *testing.T) {
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
				return []cup.Prescription{{NRE: "0A1B", Status: cup.StatusPrescribed}}, nil
			},
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
				return []cup.Appointment{{ID: "x"}}, nil
			},
		}
		f := &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) {
			return []fse.Prescription{{ID: "f1", NRE: "0a1b", Kind: fse.KindVisit, Status: 30}}, nil
		}}
		result, err := newAgenda(c, f, 1).Appointments(context.Background())
		require.NoError(t, err)
		assert.Len(t, c.AppointmentsCalls(), 1)
		require.Len(t, result.Entries, 1)
		require.NotNil(t, result.Entries[0].CUP)
		require.NotNil(t, result.Entries[0].FSE)
	})

	t.Run("expired fse session counts as empty", func(t *testing.T) {
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
				return []cup.Prescription{{NRE: "A"}}, nil
			},
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) { return nil, nil },
		}
		f := &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) {
			return nil, session.ErrSessionExpired
		}}
		_, err := newAgenda(c, f, 1).Appointments(context.Background())
		require.NoError(t, err)
		assert.Len(t, c.AppointmentsCalls(), 1)
	})

	t.Run("both portals logged out is an error", func(t *testing.T) {
		c := &MockCUP{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
			return nil, session.ErrNotLoggedIn
		}}
		f := &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) {
			return nil, session.ErrSessionExpired
		}}
		_, err := newAgenda(c, f, 1).Appointments(context.Background())
		require.ErrorIs(t, err, session.ErrNotLoggedIn)
	})

	t.Run("fse non-login error is an error", func(t *testing.T) {
		c := &MockCUP{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return nil, nil }}
		f := &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) { return nil, errBoom }}
		_, err := newAgenda(c, f, 1).Appointments(context.Background())
		require.ErrorIs(t, err, ErrFSEPrescriptions)
	})

	t.Run("prescriptions error is propagated", func(t *testing.T) {
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
				return nil, errBoom
			},
		}
		_, err := newAgenda(c, emptyFSE(), 1).Appointments(context.Background())
		require.ErrorIs(t, err, errBoom)
		assert.Empty(t, c.AppointmentsCalls())
	})

	t.Run("concurrency never exceeds the bound", func(t *testing.T) {
		var inFlight, maxInFlight atomic.Int32
		c := &MockCUP{
			PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
				return []cup.Prescription{{NRE: "1"}, {NRE: "2"}, {NRE: "3"}, {NRE: "4"}, {NRE: "5"}, {NRE: "6"}}, nil
			},
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
				current := inFlight.Add(1)
				for {
					peak := maxInFlight.Load()
					if current <= peak || maxInFlight.CompareAndSwap(peak, current) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				inFlight.Add(-1)
				return nil, nil
			},
		}
		_, err := newAgenda(c, emptyFSE(), 2).Appointments(context.Background())
		require.NoError(t, err)
		assert.Len(t, c.AppointmentsCalls(), 6)
		assert.LessOrEqual(t, maxInFlight.Load(), int32(2))
	})
}

func TestPrescriptions(t *testing.T) {
	d1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	cupReturning := func(p []cup.Prescription, err error) *MockCUP {
		return &MockCUP{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return p, err }}
	}
	fseReturning := func(p []fse.Prescription, err error) *MockFSE {
		return &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) { return p, err }}
	}

	t.Run("merges same NRE case-insensitively", func(t *testing.T) {
		c := cupReturning([]cup.Prescription{{NRE: "0A1B", PrescribedOn: d1}}, nil)
		f := fseReturning([]fse.Prescription{{ID: "f1", NRE: "0a1b", Date: d2}}, nil)
		rows, err := newAgenda(c, f, 1).Prescriptions(context.Background())
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "0A1B", rows[0].NRE)
		require.NotNil(t, rows[0].CUP)
		require.NotNil(t, rows[0].FSE)
		assert.Equal(t, "f1", rows[0].FSE.ID)
	})

	t.Run("keeps cup-only and fse-only rows", func(t *testing.T) {
		c := cupReturning([]cup.Prescription{{NRE: "C1", PrescribedOn: d1}}, nil)
		f := fseReturning([]fse.Prescription{{ID: "f1", NRE: "F1", Date: d2}}, nil)
		rows, err := newAgenda(c, f, 1).Prescriptions(context.Background())
		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.Nil(t, rows[0].CUP)
		assert.NotNil(t, rows[0].FSE)
		assert.NotNil(t, rows[1].CUP)
		assert.Nil(t, rows[1].FSE)
	})

	t.Run("empty NRE fse items stay separate", func(t *testing.T) {
		f := fseReturning([]fse.Prescription{{ID: "f1", Date: d1}, {ID: "f2", Date: d2}}, nil)
		rows, err := newAgenda(cupReturning(nil, nil), f, 1).Prescriptions(context.Background())
		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.Empty(t, rows[0].NRE)
		assert.Empty(t, rows[1].NRE)
	})

	t.Run("fse not logged in returns cup rows only", func(t *testing.T) {
		c := cupReturning([]cup.Prescription{{NRE: "C1"}}, nil)
		f := fseReturning(nil, session.ErrNotLoggedIn)
		rows, err := newAgenda(c, f, 1).Prescriptions(context.Background())
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.NotNil(t, rows[0].CUP)
	})

	t.Run("cup session expired returns fse rows only", func(t *testing.T) {
		c := cupReturning(nil, session.ErrSessionExpired)
		f := fseReturning([]fse.Prescription{{ID: "f1", NRE: "F1"}}, nil)
		rows, err := newAgenda(c, f, 1).Prescriptions(context.Background())
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.NotNil(t, rows[0].FSE)
	})

	t.Run("both login errors return the cup error", func(t *testing.T) {
		c := cupReturning(nil, session.ErrNotLoggedIn)
		f := fseReturning(nil, session.ErrSessionExpired)
		rows, err := newAgenda(c, f, 1).Prescriptions(context.Background())
		require.ErrorIs(t, err, session.ErrNotLoggedIn)
		require.ErrorIs(t, err, ErrCUPPrescriptions)
		assert.Nil(t, rows)
	})

	t.Run("other cup error is returned", func(t *testing.T) {
		c := cupReturning(nil, errBoom)
		f := fseReturning(nil, nil)
		_, err := newAgenda(c, f, 1).Prescriptions(context.Background())
		require.ErrorIs(t, err, errBoom)
	})

	t.Run("other fse error is returned", func(t *testing.T) {
		c := cupReturning([]cup.Prescription{{NRE: "C1"}}, nil)
		f := fseReturning(nil, errBoom)
		_, err := newAgenda(c, f, 1).Prescriptions(context.Background())
		require.ErrorIs(t, err, errBoom)
		require.ErrorIs(t, err, ErrFSEPrescriptions)
	})

	t.Run("sorts by newest date descending with zero last", func(t *testing.T) {
		c := cupReturning([]cup.Prescription{
			{NRE: "ZERO"},
			{NRE: "OLD", PrescribedOn: d1},
			{NRE: "MID", PrescribedOn: d2},
		}, nil)
		f := fseReturning([]fse.Prescription{{ID: "f1", NRE: "OLD", Date: d3}}, nil)
		rows, err := newAgenda(c, f, 1).Prescriptions(context.Background())
		require.NoError(t, err)
		nres := []string{}
		for _, r := range rows {
			nres = append(nres, r.NRE)
		}
		assert.Equal(t, []string{"OLD", "MID", "ZERO"}, nres)
	})
}
