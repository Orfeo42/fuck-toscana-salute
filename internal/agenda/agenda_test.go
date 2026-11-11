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

func noCUP() *MockCUP {
	return &MockCUP{
		PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return nil, nil },
		AppointmentsFunc:  func(ctx context.Context, nre string) ([]cup.Appointment, error) { return nil, nil },
	}
}

func visits(nres ...string) *MockFSE {
	return &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) {
		out := []fse.Prescription{}
		for _, nre := range nres {
			out = append(out, fse.Prescription{ID: "id-" + nre, NRE: nre, Kind: fse.KindVisit, Status: 3})
		}
		return out, nil
	}}
}

func TestAppointments(t *testing.T) {
	t.Run("filters by kind, empty NRE, closed status and age", func(t *testing.T) {
		c := noCUP()
		f := &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) {
			return []fse.Prescription{
				{ID: "f1", NRE: "OK1", Kind: fse.KindVisit, Status: 30, Date: fixedNow.AddDate(0, -1, 0)},
				{ID: "f2", NRE: "DRUG", Kind: fse.KindDrug, Status: 3},
				{ID: "f3", NRE: "", Kind: fse.KindVisit, Status: 3},
				{ID: "f4", NRE: "CANC", Kind: fse.KindVisit, Status: 4},
				{ID: "f5", NRE: "DONE", Kind: fse.KindVisit, Status: 8},
				{ID: "f6", NRE: "DONE9", Kind: fse.KindVisit, Status: 9},
				{ID: "f7", NRE: "OLD", Kind: fse.KindVisit, Status: 3, Date: fixedNow.AddDate(-2, 0, 0)},
			}, nil
		}}
		_, err := newAgenda(c, f, 2).Appointments(context.Background())
		require.NoError(t, err)
		require.Len(t, c.AppointmentsCalls(), 1)
		assert.Equal(t, "OK1", c.AppointmentsCalls()[0].Nre)
	})

	t.Run("cup-listed NRE also in fse is not looked up", func(t *testing.T) {
		c := noCUP()
		c.PrescriptionsFunc = func(ctx context.Context) ([]cup.Prescription, error) {
			return []cup.Prescription{{NRE: "0A1B", Status: cup.StatusPrescribed}}, nil
		}
		result, err := newAgenda(c, visits("0a1b", "F2"), 1).Appointments(context.Background())
		require.NoError(t, err)
		require.Len(t, c.AppointmentsCalls(), 1)
		assert.Equal(t, "F2", c.AppointmentsCalls()[0].Nre)
		assert.Empty(t, result.Entries)
	})

	t.Run("cup-only NRE is not looked up", func(t *testing.T) {
		c := noCUP()
		c.PrescriptionsFunc = func(ctx context.Context) ([]cup.Prescription, error) {
			return []cup.Prescription{{NRE: "C1", Status: cup.StatusPrescribed}, {NRE: "C2", Status: cup.StatusBooked}}, nil
		}
		_, err := newAgenda(c, emptyFSE(), 1).Appointments(context.Background())
		require.NoError(t, err)
		assert.Empty(t, c.AppointmentsCalls())
	})

	t.Run("fse-only visit NRE is looked up with nil cup", func(t *testing.T) {
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			return []cup.Appointment{{ID: "a-" + nre, Status: "PRENOTATO"}}, nil
		}
		result, err := newAgenda(c, visits("F1"), 1).Appointments(context.Background())
		require.NoError(t, err)
		require.Len(t, result.Entries, 1)
		assert.Equal(t, "F1", result.Entries[0].NRE)
		assert.Nil(t, result.Entries[0].CUP)
		require.NotNil(t, result.Entries[0].FSE)
		assert.Equal(t, "id-F1", result.Entries[0].FSE.ID)
	})

	t.Run("duplicate fse NRE is looked up once", func(t *testing.T) {
		c := noCUP()
		_, err := newAgenda(c, visits("0A1B", "0a1b"), 1).Appointments(context.Background())
		require.NoError(t, err)
		assert.Len(t, c.AppointmentsCalls(), 1)
	})

	t.Run("sorts booked first then date ascending with zero last", func(t *testing.T) {
		d1 := fixedNow.AddDate(0, 0, 1)
		d2 := fixedNow.AddDate(0, 0, 2)
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			return []cup.Appointment{
				{ID: "cancelled-early", Status: "DISDETTO", At: d1},
				{ID: "booked-zero", Status: "PRENOTATO"},
				{ID: "booked-late", Status: "PRENOTATO", At: d2},
				{ID: "booked-early", Status: "PRENOTATO", At: d1},
				{ID: "cancelled-zero", Status: "DISDETTO"},
			}, nil
		}
		result, err := newAgenda(c, visits("A"), 1).Appointments(context.Background())
		require.NoError(t, err)
		ids := []string{}
		for _, e := range result.Entries {
			ids = append(ids, e.Appointment.ID)
			assert.Equal(t, "A", e.NRE)
		}
		assert.Equal(t, []string{"booked-early", "booked-late", "booked-zero", "cancelled-early", "cancelled-zero"}, ids)
	})

	t.Run("one failing lookup is recorded and others succeed", func(t *testing.T) {
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			if nre == "B" {
				return nil, errBoom
			}
			return []cup.Appointment{{ID: "id-" + nre, Status: "PRENOTATO"}}, nil
		}
		result, err := newAgenda(c, visits("A", "B", "C"), 1).Appointments(context.Background())
		require.NoError(t, err)
		require.Len(t, result.Entries, 2)
		assert.Equal(t, "id-A", result.Entries[0].Appointment.ID)
		assert.Equal(t, "id-C", result.Entries[1].Appointment.ID)
		require.Len(t, result.Failed, 1)
		assert.Equal(t, "B", result.Failed[0].NRE)
		assert.Nil(t, result.Failed[0].CUP)
		require.ErrorIs(t, result.Failed[0].Err, errBoom)
		assert.Len(t, c.AppointmentsCalls(), 3)
	})

	t.Run("all lookups failing returns failures and no error", func(t *testing.T) {
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			return nil, errBoom
		}
		result, err := newAgenda(c, visits("A", "B"), 2).Appointments(context.Background())
		require.NoError(t, err)
		assert.Empty(t, result.Entries)
		require.Len(t, result.Failed, 2)
		assert.Equal(t, "A", result.Failed[0].NRE)
		assert.Equal(t, "B", result.Failed[1].NRE)
	})

	t.Run("cancelled context fails the call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			cancel()
			return nil, ctx.Err()
		}
		_, err := newAgenda(c, visits("A"), 1).Appointments(ctx)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("expired cup session still looks up fse visits", func(t *testing.T) {
		c := noCUP()
		c.PrescriptionsFunc = func(ctx context.Context) ([]cup.Prescription, error) {
			return nil, session.ErrSessionExpired
		}
		_, err := newAgenda(c, visits("A"), 1).Appointments(context.Background())
		require.NoError(t, err)
		assert.Len(t, c.AppointmentsCalls(), 1)
	})

	t.Run("expired fse session means no candidates", func(t *testing.T) {
		c := noCUP()
		f := &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) {
			return nil, session.ErrSessionExpired
		}}
		_, err := newAgenda(c, f, 1).Appointments(context.Background())
		require.NoError(t, err)
		assert.Empty(t, c.AppointmentsCalls())
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
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
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
		}
		_, err := newAgenda(c, visits("1", "2", "3", "4", "5", "6"), 2).Appointments(context.Background())
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
