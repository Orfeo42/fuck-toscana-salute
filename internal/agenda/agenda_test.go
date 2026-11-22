package agenda

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/cache"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/store"
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
		VerdictFunc: func(ctx context.Context, nre string) (cup.Verdict, error) {
			return cup.Verdict{Reason: cup.VerdictBookable}, nil
		},
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

	t.Run("unbooked lists NREs with zero appointments in candidate order", func(t *testing.T) {
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			if nre == "B" {
				return []cup.Appointment{{ID: "id-B", Status: "PRENOTATO"}}, nil
			}
			if nre == "D" {
				return nil, errBoom
			}
			return nil, nil
		}
		result, err := newAgenda(c, visits("A", "B", "C", "D"), 2).Appointments(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []string{"A", "C"}, result.Unbooked)
		require.Len(t, result.Failed, 1)
		assert.Equal(t, "D", result.Failed[0].NRE)
	})

	t.Run("verdicts fetched for every unbooked NRE keyed by upper NRE", func(t *testing.T) {
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			if nre == "b" {
				return []cup.Appointment{{ID: "id-b", Status: "PRENOTATO"}}, nil
			}
			return nil, nil
		}
		c.VerdictFunc = func(ctx context.Context, nre string) (cup.Verdict, error) {
			return cup.Verdict{Reason: cup.VerdictHandled, Message: "m-" + nre}, nil
		}
		result, err := newAgenda(c, visits("a", "b", "c"), 2).Appointments(context.Background())
		require.NoError(t, err)
		assert.Len(t, c.VerdictCalls(), 2)
		require.Len(t, result.Verdicts, 2)
		assert.Equal(t, "m-a", result.Verdicts["A"].Message)
		assert.Equal(t, cup.VerdictHandled, result.Verdicts["C"].Reason)
		assert.NotContains(t, result.Verdicts, "B")
	})

	t.Run("verdict error leaves the NRE out without failing", func(t *testing.T) {
		c := noCUP()
		c.VerdictFunc = func(ctx context.Context, nre string) (cup.Verdict, error) {
			if nre == "B" {
				return cup.Verdict{}, errBoom
			}
			return cup.Verdict{Reason: cup.VerdictBookable}, nil
		}
		result, err := newAgenda(c, visits("A", "B"), 1).Appointments(context.Background())
		require.NoError(t, err)
		assert.Equal(t, []string{"A", "B"}, result.Unbooked)
		assert.Contains(t, result.Verdicts, "A")
		assert.NotContains(t, result.Verdicts, "B")
	})

	t.Run("verdicts is empty non-nil without unbooked", func(t *testing.T) {
		result, err := newAgenda(noCUP(), emptyFSE(), 1).Appointments(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, result.Verdicts)
		assert.Empty(t, result.Verdicts)
	})

	t.Run("unbooked is empty non-nil without candidates", func(t *testing.T) {
		result, err := newAgenda(noCUP(), emptyFSE(), 1).Appointments(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, result.Unbooked)
		assert.Empty(t, result.Unbooked)
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

func cachedCUP(list cache.Entry[[]cup.Prescription], listErr error, appointments map[string]cache.Entry[[]cup.Appointment], verdicts map[string]cache.Entry[cup.Verdict]) *MockCUP {
	return &MockCUP{
		CachedPrescriptionsFunc: func(ctx context.Context) (cache.Entry[[]cup.Prescription], error) { return list, listErr },
		CachedAppointmentsFunc: func(ctx context.Context) (map[string]cache.Entry[[]cup.Appointment], error) {
			return appointments, nil
		},
		CachedVerdictsFunc: func(ctx context.Context) (map[string]cache.Entry[cup.Verdict], error) { return verdicts, nil },
	}
}

func cachedFSE(list cache.Entry[[]fse.Prescription], err error) *MockFSE {
	return &MockFSE{CachedPrescriptionsFunc: func(ctx context.Context) (cache.Entry[[]fse.Prescription], error) { return list, err }}
}

func TestSnapshot(t *testing.T) {
	at := fixedNow.Add(-10 * time.Minute)
	visit := func(nre string, status int) fse.Prescription {
		return fse.Prescription{ID: "id-" + nre, NRE: nre, Kind: fse.KindVisit, Status: status}
	}

	t.Run("reads only the store four times and never upstream", func(t *testing.T) {
		c := cachedCUP(cache.Entry[[]cup.Prescription]{}, cache.ErrNotCached, nil, nil)
		f := cachedFSE(cache.Entry[[]fse.Prescription]{Value: []fse.Prescription{visit("A1", 3), visit("A2", 3), visit("A3", 3)}, FetchedAt: at}, nil)
		_, err := newAgenda(c, f, 2).Snapshot(context.Background())
		require.NoError(t, err)
		assert.Len(t, c.CachedPrescriptionsCalls(), 1)
		assert.Len(t, c.CachedAppointmentsCalls(), 1)
		assert.Len(t, c.CachedVerdictsCalls(), 1)
		assert.Len(t, f.CachedPrescriptionsCalls(), 1)
		assert.Empty(t, c.PrescriptionsCalls())
		assert.Empty(t, c.AppointmentsCalls())
		assert.Empty(t, c.VerdictCalls())
		assert.Empty(t, f.PrescriptionsCalls())
	})

	t.Run("missing lists are flagged and rows are empty", func(t *testing.T) {
		c := cachedCUP(cache.Entry[[]cup.Prescription]{}, cache.ErrNotCached, nil, nil)
		f := cachedFSE(cache.Entry[[]fse.Prescription]{}, cache.ErrNotCached)
		snapshot, err := newAgenda(c, f, 1).Snapshot(context.Background())
		require.NoError(t, err)
		assert.True(t, snapshot.CUPList.Missing)
		assert.True(t, snapshot.FSEList.Missing)
		assert.Empty(t, snapshot.Rows)
		assert.Empty(t, snapshot.Lookups)
	})

	t.Run("list state carries time and stale flag", func(t *testing.T) {
		c := cachedCUP(cache.Entry[[]cup.Prescription]{FetchedAt: at, Stale: true}, nil, nil, nil)
		f := cachedFSE(cache.Entry[[]fse.Prescription]{FetchedAt: at}, nil)
		snapshot, err := newAgenda(c, f, 1).Snapshot(context.Background())
		require.NoError(t, err)
		assert.Equal(t, ListState{At: at, Stale: true}, snapshot.CUPList)
		assert.Equal(t, ListState{At: at}, snapshot.FSEList)
	})

	t.Run("other list error is returned", func(t *testing.T) {
		c := cachedCUP(cache.Entry[[]cup.Prescription]{}, errBoom, nil, nil)
		_, err := newAgenda(c, cachedFSE(cache.Entry[[]fse.Prescription]{}, nil), 1).Snapshot(context.Background())
		require.ErrorIs(t, err, errBoom)
		require.ErrorIs(t, err, ErrCUPPrescriptions)
	})

	t.Run("fse list error is returned", func(t *testing.T) {
		c := cachedCUP(cache.Entry[[]cup.Prescription]{}, nil, nil, nil)
		_, err := newAgenda(c, cachedFSE(cache.Entry[[]fse.Prescription]{}, errBoom), 1).Snapshot(context.Background())
		require.ErrorIs(t, err, ErrFSEPrescriptions)
	})

	t.Run("appointment and verdict read errors are returned", func(t *testing.T) {
		c := cachedCUP(cache.Entry[[]cup.Prescription]{}, nil, nil, nil)
		c.CachedAppointmentsFunc = func(ctx context.Context) (map[string]cache.Entry[[]cup.Appointment], error) { return nil, errBoom }
		_, err := newAgenda(c, cachedFSE(cache.Entry[[]fse.Prescription]{}, nil), 1).Snapshot(context.Background())
		require.ErrorIs(t, err, ErrAppointments)

		c = cachedCUP(cache.Entry[[]cup.Prescription]{}, nil, nil, nil)
		c.CachedVerdictsFunc = func(ctx context.Context) (map[string]cache.Entry[cup.Verdict], error) { return nil, errBoom }
		_, err = newAgenda(c, cachedFSE(cache.Entry[[]fse.Prescription]{}, nil), 1).Snapshot(context.Background())
		require.ErrorIs(t, err, ErrVerdicts)
	})

	t.Run("lookups follow the candidates rules", func(t *testing.T) {
		cupList := []cup.Prescription{{NRE: "0a1b", Status: cup.StatusPrescribed}}
		fseList := []fse.Prescription{
			visit("0A1B", 3),
			visit("OK1", 30),
			{ID: "d", NRE: "DRUG", Kind: fse.KindDrug, Status: 3},
			{ID: "e", NRE: "", Kind: fse.KindVisit, Status: 3},
			visit("CANC", 4),
			visit("DONE", 8),
			visit("DONE9", 9),
			{ID: "old", NRE: "OLD", Kind: fse.KindVisit, Status: 3, Date: fixedNow.AddDate(-2, 0, 0)},
		}
		c := cachedCUP(cache.Entry[[]cup.Prescription]{Value: cupList, FetchedAt: at}, nil, nil, nil)
		f := cachedFSE(cache.Entry[[]fse.Prescription]{Value: fseList, FetchedAt: at}, nil)
		ag := newAgenda(c, f, 1)
		snapshot, err := ag.Snapshot(context.Background())
		require.NoError(t, err)

		expected := map[string]bool{}
		for _, cand := range ag.candidates(cupList, fseList) {
			expected[strings.ToUpper(cand.nre)] = true
		}
		got := map[string]bool{}
		for key, lookup := range snapshot.Lookups {
			got[key] = true
			assert.True(t, lookup.Candidate)
		}
		assert.Equal(t, expected, got)
		assert.Equal(t, map[string]bool{"OK1": true}, got)
	})

	t.Run("lookup carries appointments, check time and stale flag", func(t *testing.T) {
		appointments := map[string]cache.Entry[[]cup.Appointment]{
			"A1": {Value: []cup.Appointment{{ID: "x", Status: bookedStatus}}, FetchedAt: at, Stale: true},
		}
		c := cachedCUP(cache.Entry[[]cup.Prescription]{}, nil, appointments, nil)
		f := cachedFSE(cache.Entry[[]fse.Prescription]{Value: []fse.Prescription{visit("a1", 3)}, FetchedAt: at}, nil)
		snapshot, err := newAgenda(c, f, 1).Snapshot(context.Background())
		require.NoError(t, err)
		lookup := snapshot.Lookups["A1"]
		assert.True(t, lookup.Checked)
		assert.True(t, lookup.Stale)
		assert.Equal(t, at, lookup.CheckedAt)
		require.Len(t, lookup.Appointments, 1)
		assert.Nil(t, lookup.Verdict)
	})

	t.Run("unchecked candidate is not Checked", func(t *testing.T) {
		c := cachedCUP(cache.Entry[[]cup.Prescription]{}, nil, map[string]cache.Entry[[]cup.Appointment]{}, nil)
		f := cachedFSE(cache.Entry[[]fse.Prescription]{Value: []fse.Prescription{visit("A1", 3)}, FetchedAt: at}, nil)
		snapshot, err := newAgenda(c, f, 1).Snapshot(context.Background())
		require.NoError(t, err)
		assert.Equal(t, Lookup{Candidate: true}, snapshot.Lookups["A1"])
	})

	t.Run("unbooked candidate carries its verdict", func(t *testing.T) {
		verdictAt := fixedNow.Add(-time.Hour)
		appointments := map[string]cache.Entry[[]cup.Appointment]{"A1": {Value: []cup.Appointment{}, FetchedAt: at}}
		verdicts := map[string]cache.Entry[cup.Verdict]{"A1": {Value: cup.Verdict{Reason: cup.VerdictHandled}, FetchedAt: verdictAt, Stale: true}}
		c := cachedCUP(cache.Entry[[]cup.Prescription]{}, nil, appointments, verdicts)
		f := cachedFSE(cache.Entry[[]fse.Prescription]{Value: []fse.Prescription{visit("A1", 3)}, FetchedAt: at}, nil)
		snapshot, err := newAgenda(c, f, 1).Snapshot(context.Background())
		require.NoError(t, err)
		lookup := snapshot.Lookups["A1"]
		require.NotNil(t, lookup.Verdict)
		assert.Equal(t, cup.VerdictHandled, lookup.Verdict.Verdict.Reason)
		assert.Equal(t, verdictAt, lookup.Verdict.VerifiedAt)
		assert.True(t, lookup.Verdict.Stale)
	})

	t.Run("rows match the list merge", func(t *testing.T) {
		cupList := []cup.Prescription{{NRE: "0A1B", PrescribedOn: fixedNow.AddDate(0, -3, 0)}}
		fseList := []fse.Prescription{{ID: "f1", NRE: "0a1b", Date: fixedNow.AddDate(0, -2, 0)}, {ID: "f2", NRE: "F2", Date: fixedNow.AddDate(0, -1, 0)}}
		c := cachedCUP(cache.Entry[[]cup.Prescription]{Value: cupList, FetchedAt: at}, nil, nil, nil)
		f := cachedFSE(cache.Entry[[]fse.Prescription]{Value: fseList, FetchedAt: at}, nil)
		snapshot, err := newAgenda(c, f, 1).Snapshot(context.Background())
		require.NoError(t, err)
		require.Len(t, snapshot.Rows, 2)
		assert.Equal(t, "F2", snapshot.Rows[0].NRE)
		assert.Equal(t, "0A1B", snapshot.Rows[1].NRE)
		assert.NotNil(t, snapshot.Rows[1].CUP)
		assert.NotNil(t, snapshot.Rows[1].FSE)
	})
}

func TestLists(t *testing.T) {
	t.Run("refreshes both prescription lists only", func(t *testing.T) {
		c := noCUP()
		f := visits("A1")
		require.NoError(t, newAgenda(c, f, 1).Lists(context.Background()))
		assert.Len(t, c.PrescriptionsCalls(), 1)
		assert.Len(t, f.PrescriptionsCalls(), 1)
		assert.Empty(t, c.AppointmentsCalls())
		assert.Empty(t, c.VerdictCalls())
	})

	t.Run("both login errors fail", func(t *testing.T) {
		c := noCUP()
		c.PrescriptionsFunc = func(ctx context.Context) ([]cup.Prescription, error) { return nil, session.ErrNotLoggedIn }
		f := &MockFSE{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) { return nil, session.ErrNotLoggedIn }}
		require.ErrorIs(t, newAgenda(c, f, 1).Lists(context.Background()), session.ErrNotLoggedIn)
	})
}

func TestRefreshNREs(t *testing.T) {
	t.Run("touches only the given NREs", func(t *testing.T) {
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			if nre == "B2" {
				return []cup.Appointment{{ID: "x"}}, nil
			}
			return nil, nil
		}
		f := visits("A1", "B2", "C3")
		require.NoError(t, newAgenda(c, f, 2).RefreshNREs(context.Background(), []string{"A1", "B2"}))
		got := []string{}
		for _, call := range c.AppointmentsCalls() {
			got = append(got, call.Nre)
		}
		assert.ElementsMatch(t, []string{"A1", "B2"}, got)
		require.Len(t, c.VerdictCalls(), 1)
		assert.Equal(t, "A1", c.VerdictCalls()[0].Nre)
		assert.Empty(t, c.PrescriptionsCalls())
		assert.Empty(t, f.PrescriptionsCalls())
	})

	t.Run("forces the real cache for the given NRE only", func(t *testing.T) {
		st := &cache.MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				payload, err := json.Marshal([]cup.Appointment{})
				require.NoError(t, err)
				return store.Record{Source: source, Subject: subject, FetchedAt: time.Now(), Payload: payload}, nil
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &cache.MockCUPSource{
			AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
				return []cup.Appointment{{ID: "x"}}, nil
			},
		}
		ag := New(cache.NewCUP(src, st, cache.DefaultPolicy()), emptyFSE(), 2)
		require.NoError(t, ag.RefreshNREs(context.Background(), []string{"a1"}))
		require.Len(t, src.AppointmentsCalls(), 1)
		assert.Equal(t, "A1", src.AppointmentsCalls()[0].Nre)

		_, err := ag.cup.Appointments(context.Background(), "B2")
		require.NoError(t, err)
		assert.Len(t, src.AppointmentsCalls(), 1)
	})

	t.Run("lookup failure is joined and keeps its cause", func(t *testing.T) {
		c := noCUP()
		c.AppointmentsFunc = func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			return nil, session.ErrSessionExpired
		}
		err := newAgenda(c, emptyFSE(), 1).RefreshNREs(context.Background(), []string{"A1"})
		require.ErrorIs(t, err, session.ErrSessionExpired)
		require.ErrorIs(t, err, ErrRefreshNREs)
		assert.Empty(t, c.VerdictCalls())
	})
}

func TestBestBooking(t *testing.T) {
	early := time.Date(2026, 11, 12, 9, 30, 0, 0, time.UTC)
	late := time.Date(2026, 12, 12, 9, 30, 0, 0, time.UTC)

	t.Run("empty input has no booking", func(t *testing.T) {
		_, ok := BestBooking(nil)
		assert.False(t, ok)
	})

	t.Run("prefers a live booking over a later cancelled one", func(t *testing.T) {
		got, ok := BestBooking([]cup.Appointment{{ID: "b", Status: "DISDETTO", At: late}, {ID: "a", Status: bookedStatus, At: early}})
		require.True(t, ok)
		assert.Equal(t, "a", got.ID)
	})

	t.Run("prefers the latest among live bookings", func(t *testing.T) {
		got, _ := BestBooking([]cup.Appointment{{ID: "b", Status: bookedStatus, At: late}, {ID: "a", Status: bookedStatus, At: early}})
		assert.Equal(t, "b", got.ID)
		got, _ = BestBooking([]cup.Appointment{{ID: "a", Status: bookedStatus, At: early}, {ID: "b", Status: bookedStatus, At: late}})
		assert.Equal(t, "b", got.ID)
	})

	t.Run("falls back to the latest cancelled booking", func(t *testing.T) {
		got, _ := BestBooking([]cup.Appointment{{ID: "a", Status: "DISDETTO", At: early}, {ID: "b", Status: "DISDETTO", At: late}})
		assert.Equal(t, "b", got.ID)
	})

	t.Run("a booking without date is the oldest", func(t *testing.T) {
		got, _ := BestBooking([]cup.Appointment{{ID: "b", Status: bookedStatus, At: early}, {ID: "a", Status: bookedStatus}})
		assert.Equal(t, "b", got.ID)
		got, _ = BestBooking([]cup.Appointment{{ID: "a", Status: bookedStatus}, {ID: "b", Status: bookedStatus, At: early}})
		assert.Equal(t, "b", got.ID)
	})

	t.Run("a later live booking replaces an earlier cancelled one", func(t *testing.T) {
		got, _ := BestBooking([]cup.Appointment{{ID: "a", Status: "DISDETTO", At: late}, {ID: "b", Status: bookedStatus, At: early}})
		assert.Equal(t, "b", got.ID)
	})

	t.Run("equal bookings keep the first", func(t *testing.T) {
		got, _ := BestBooking([]cup.Appointment{{ID: "a", Status: bookedStatus, At: early}, {ID: "b", Status: bookedStatus, At: early}})
		assert.Equal(t, "a", got.ID)
	})
}

func TestRowHelpers(t *testing.T) {
	d1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	t.Run("title prefers the fse description", func(t *testing.T) {
		r := Row{FSE: &fse.Prescription{Description: "Visita"}, CUP: &cup.Prescription{Items: []string{"X"}}}
		assert.Equal(t, "Visita", r.Title())
	})

	t.Run("title joins cup items when fse description is empty", func(t *testing.T) {
		r := Row{FSE: &fse.Prescription{}, CUP: &cup.Prescription{Items: []string{"ECG", "VISITA"}}}
		assert.Equal(t, "ECG, VISITA", r.Title())
	})

	t.Run("title falls back to Prescrizione", func(t *testing.T) {
		assert.Equal(t, "Prescrizione", Row{CUP: &cup.Prescription{}}.Title())
		assert.Equal(t, "Prescrizione", Row{}.Title())
	})

	t.Run("prescribed date prefers fse over cup", func(t *testing.T) {
		r := Row{FSE: &fse.Prescription{Date: d2}, CUP: &cup.Prescription{PrescribedOn: d1}}
		assert.Equal(t, d2, r.PrescribedOn())
	})

	t.Run("prescribed date falls back to cup then zero", func(t *testing.T) {
		assert.Equal(t, d1, Row{FSE: &fse.Prescription{}, CUP: &cup.Prescription{PrescribedOn: d1}}.PrescribedOn())
		assert.True(t, Row{}.PrescribedOn().IsZero())
	})

	t.Run("fse visit is a visit and fse drug is not", func(t *testing.T) {
		assert.True(t, Row{FSE: &fse.Prescription{Kind: fse.KindVisit}}.IsVisit())
		assert.False(t, Row{FSE: &fse.Prescription{Kind: fse.KindDrug}}.IsVisit())
	})

	t.Run("fse drug with cup is not a visit", func(t *testing.T) {
		assert.False(t, Row{FSE: &fse.Prescription{Kind: fse.KindDrug}, CUP: &cup.Prescription{}}.IsVisit())
	})

	t.Run("cup-only row is a visit", func(t *testing.T) {
		assert.True(t, Row{CUP: &cup.Prescription{}}.IsVisit())
		assert.False(t, Row{}.IsVisit())
	})
}
