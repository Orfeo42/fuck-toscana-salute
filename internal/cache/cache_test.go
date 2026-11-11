package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/store"
)

const testNRE = "0900A0000000001"

var testNow = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

func fakePrescriptions() []cup.Prescription {
	return []cup.Prescription{{
		NRE:          testNRE,
		PrescribedOn: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Kind:         "SPECIALISTICA",
		Status:       cup.StatusPrescribed,
		Items:        []string{"VISITA", "ECG"},
	}}
}

func fakeAppointments() []cup.Appointment {
	return []cup.Appointment{{
		ID:        "A1",
		BookingID: "B1",
		NRE:       testNRE,
		Status:    "PRENOTATO",
		At:        time.Date(2026, 12, 1, 9, 30, 0, 0, time.UTC),
		Unit: cup.Unit{
			Name: "U", Description: "D", Place: "P", Address: "Via X 1",
			PostalCode: "50100", City: "Firenze", Province: "FI",
		},
	}}
}

func fakeFSE() []fse.Prescription {
	return []fse.Prescription{{
		ID:              "F1",
		NRE:             testNRE,
		Description:     "desc",
		Kind:            fse.Kind("x"),
		Status:          2,
		Date:            time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		DocumentRef:     "ref",
		ExpiresAt:       time.Date(2027, 9, 1, 0, 0, 0, 0, time.UTC),
		ExpiryEstimated: true,
	}}
}

func record(t *testing.T, source, subject string, fetched time.Time, v any) store.Record {
	t.Helper()
	payload, err := json.Marshal(v)
	require.NoError(t, err)
	return store.Record{Source: source, Subject: subject, FetchedAt: fetched, Status: 200, Payload: payload}
}

func newCUP(src CUPSource, st Store) *CUP {
	c := NewCUP(src, st, DefaultPolicy())
	c.now = func() time.Time { return testNow }
	return c
}

func TestDefaultPolicy(t *testing.T) {
	t.Run("values", func(t *testing.T) {
		p := DefaultPolicy()
		assert.Equal(t, time.Hour, p.Lists)
		assert.Equal(t, time.Hour, p.Appointments)
		assert.Equal(t, 24*time.Hour, p.Verdicts)
	})
}

func TestCUPPrescriptions(t *testing.T) {
	want := fakePrescriptions()

	t.Run("fresh hit skips source", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow.Add(-10*time.Minute), want), nil
		}}
		src := &MockCUPSource{}
		got, err := newCUP(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Empty(t, src.PrescriptionsCalls())
		assert.Empty(t, st.PutCalls())
	})

	t.Run("expired calls source and stores", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return record(t, source, subject, testNow.Add(-2*time.Hour), []cup.Prescription{}), nil
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockCUPSource{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return want, nil }}
		got, err := newCUP(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
		require.Len(t, st.PutCalls(), 1)
		put := st.PutCalls()[0].Rec
		assert.Equal(t, SourceCUPPrescriptions, put.Source)
		assert.Equal(t, "", put.Subject)
		assert.Equal(t, testNow, put.FetchedAt)
		assert.Equal(t, 200, put.Status)
	})

	t.Run("miss calls source and stores", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return store.Record{}, store.ErrNotFound
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockCUPSource{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return want, nil }}
		got, err := newCUP(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Len(t, st.PutCalls(), 1)
	})

	t.Run("forced by All bypasses fresh cache", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return record(t, source, subject, testNow, []cup.Prescription{}), nil
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockCUPSource{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return want, nil }}
		ctx := WithRefresh(context.Background(), Scope{All: true})
		got, err := newCUP(src, st).Prescriptions(ctx)
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Len(t, src.PrescriptionsCalls(), 1)
	})

	t.Run("list not forced by NRE scope", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow, want), nil
		}}
		src := &MockCUPSource{}
		ctx := WithRefresh(context.Background(), Scope{NREs: []string{testNRE}})
		got, err := newCUP(src, st).Prescriptions(ctx)
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Empty(t, src.PrescriptionsCalls())
	})

	t.Run("source error with stale serves stale", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow.Add(-48*time.Hour), want), nil
		}}
		src := &MockCUPSource{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
			return nil, session.ErrNotLoggedIn
		}}
		got, err := newCUP(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Empty(t, st.PutCalls())
	})

	t.Run("source error without record keeps error", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return store.Record{}, store.ErrNotFound
		}}
		src := &MockCUPSource{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) {
			return nil, session.ErrNotLoggedIn
		}}
		_, err := newCUP(src, st).Prescriptions(context.Background())
		require.Error(t, err)
		assert.True(t, errors.Is(err, session.ErrNotLoggedIn))
	})

	t.Run("put failure still returns value", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return store.Record{}, store.ErrNotFound
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return errors.New("db down") },
		}
		src := &MockCUPSource{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return want, nil }}
		got, err := newCUP(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("store read error treated as miss", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return store.Record{}, errors.New("boom")
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockCUPSource{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return want, nil }}
		got, err := newCUP(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Len(t, src.PrescriptionsCalls(), 1)
	})

	t.Run("undecodable payload treated as miss", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return store.Record{Source: source, FetchedAt: testNow, Payload: []byte("not json")}, nil
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockCUPSource{PrescriptionsFunc: func(ctx context.Context) ([]cup.Prescription, error) { return want, nil }}
		got, err := newCUP(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("round trip", func(t *testing.T) {
		raw, err := json.Marshal(want)
		require.NoError(t, err)
		var got []cup.Prescription
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, want, got)
	})
}

func TestCUPAppointments(t *testing.T) {
	want := fakeAppointments()

	t.Run("fresh hit uses upper-cased subject", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow.Add(-time.Minute), want), nil
		}}
		src := &MockCUPSource{}
		got, err := newCUP(src, st).Appointments(context.Background(), "0900a0000000001")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Empty(t, src.AppointmentsCalls())
		assert.Equal(t, testNRE, st.LatestCalls()[0].Subject)
		assert.Equal(t, SourceCUPAppointments, st.LatestCalls()[0].Source)
	})

	t.Run("forced by NRE case-insensitive", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return record(t, source, subject, testNow, []cup.Appointment{}), nil
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockCUPSource{AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) { return want, nil }}
		ctx := WithRefresh(context.Background(), Scope{NREs: []string{"0900a0000000001"}})
		got, err := newCUP(src, st).Appointments(ctx, testNRE)
		require.NoError(t, err)
		assert.Equal(t, want, got)
		require.Len(t, src.AppointmentsCalls(), 1)
		assert.Equal(t, testNRE, src.AppointmentsCalls()[0].Nre)
	})

	t.Run("other NRE in scope does not force", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow, want), nil
		}}
		src := &MockCUPSource{}
		ctx := WithRefresh(context.Background(), Scope{NREs: []string{"0900A0000000002"}})
		_, err := newCUP(src, st).Appointments(ctx, testNRE)
		require.NoError(t, err)
		assert.Empty(t, src.AppointmentsCalls())
	})

	t.Run("forced with source error falls back to stale", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow, want), nil
		}}
		src := &MockCUPSource{AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			return nil, cup.ErrGestioneNotReady
		}}
		ctx := WithRefresh(context.Background(), Scope{All: true})
		got, err := newCUP(src, st).Appointments(ctx, testNRE)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("source error without record keeps error", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return store.Record{}, store.ErrNotFound
		}}
		src := &MockCUPSource{AppointmentsFunc: func(ctx context.Context, nre string) ([]cup.Appointment, error) {
			return nil, cup.ErrGestioneNotReady
		}}
		_, err := newCUP(src, st).Appointments(context.Background(), testNRE)
		assert.True(t, errors.Is(err, cup.ErrGestioneNotReady))
	})

	t.Run("round trip", func(t *testing.T) {
		raw, err := json.Marshal(want)
		require.NoError(t, err)
		var got []cup.Appointment
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, want, got)
	})
}

func TestCUPVerdict(t *testing.T) {
	rawWithPatient := []byte(`{"message":{"level":"SUCCESS"},"response":{"paziente":{"cf":"X"},"prescrizione":{"nre":"0900A0000000001"}}}`)
	want := cup.Verdict{Reason: cup.VerdictBookable, Message: "ok", HTTPStatus: 200, Raw: rawWithPatient}

	t.Run("paziente stripped and other fields kept", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return store.Record{}, store.ErrNotFound
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockCUPSource{VerdictFunc: func(ctx context.Context, nre string) (cup.Verdict, error) { return want, nil }}
		got, err := newCUP(src, st).Verdict(context.Background(), testNRE)
		require.NoError(t, err)
		assert.Equal(t, cup.VerdictBookable, got.Reason)
		assert.Equal(t, "ok", got.Message)

		require.Len(t, st.PutCalls(), 1)
		var stored cup.Verdict
		require.NoError(t, json.Unmarshal(st.PutCalls()[0].Rec.Payload, &stored))
		assert.Equal(t, "ok", stored.Message)
		assert.NotContains(t, string(stored.Raw), "paziente")
		assert.JSONEq(t, `{"message":{"level":"SUCCESS"},"response":{"prescrizione":{"nre":"0900A0000000001"}}}`, string(stored.Raw))
	})

	t.Run("raw of unexpected shape kept", func(t *testing.T) {
		assert.Equal(t, []byte("not json"), stripPatient([]byte("not json")))
		assert.Equal(t, []byte(`{"response":"x"}`), stripPatient([]byte(`{"response":"x"}`)))
		assert.Equal(t, []byte(`{"a":1}`), stripPatient([]byte(`{"a":1}`)))
		assert.Equal(t, []byte(`{"response":{"b":1}}`), stripPatient([]byte(`{"response":{"b":1}}`)))
	})

	t.Run("fresh hit skips source", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow.Add(-23*time.Hour), want), nil
		}}
		src := &MockCUPSource{}
		got, err := newCUP(src, st).Verdict(context.Background(), testNRE)
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Empty(t, src.VerdictCalls())
	})

	t.Run("expired after 24h", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return record(t, source, subject, testNow.Add(-25*time.Hour), want), nil
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockCUPSource{VerdictFunc: func(ctx context.Context, nre string) (cup.Verdict, error) { return want, nil }}
		_, err := newCUP(src, st).Verdict(context.Background(), testNRE)
		require.NoError(t, err)
		assert.Len(t, src.VerdictCalls(), 1)
	})

	t.Run("source error with stale serves stale", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow.Add(-72*time.Hour), want), nil
		}}
		src := &MockCUPSource{VerdictFunc: func(ctx context.Context, nre string) (cup.Verdict, error) {
			return cup.Verdict{}, session.ErrNotLoggedIn
		}}
		got, err := newCUP(src, st).Verdict(context.Background(), testNRE)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("round trip", func(t *testing.T) {
		raw, err := json.Marshal(want)
		require.NoError(t, err)
		var got cup.Verdict
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, want, got)
	})
}

func TestFSEPrescriptions(t *testing.T) {
	want := fakeFSE()
	newFSE := func(src FSESource, st Store) *FSE {
		f := NewFSE(src, st, DefaultPolicy())
		f.now = func() time.Time { return testNow }
		return f
	}

	t.Run("fresh hit skips source", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow.Add(-time.Minute), want), nil
		}}
		src := &MockFSESource{}
		got, err := newFSE(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Empty(t, src.PrescriptionsCalls())
		assert.Equal(t, SourceFSEPrescriptions, st.LatestCalls()[0].Source)
	})

	t.Run("miss calls source and stores", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return store.Record{}, store.ErrNotFound
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockFSESource{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) { return want, nil }}
		got, err := newFSE(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.Len(t, st.PutCalls(), 1)
	})

	t.Run("forced by All", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return record(t, source, subject, testNow, []fse.Prescription{}), nil
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockFSESource{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) { return want, nil }}
		ctx := WithRefresh(context.Background(), Scope{All: true})
		got, err := newFSE(src, st).Prescriptions(ctx)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("source error with stale serves stale", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow.Add(-5*time.Hour), want), nil
		}}
		src := &MockFSESource{PrescriptionsFunc: func(ctx context.Context) ([]fse.Prescription, error) {
			return nil, session.ErrNotLoggedIn
		}}
		got, err := newFSE(src, st).Prescriptions(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("round trip", func(t *testing.T) {
		raw, err := json.Marshal(want)
		require.NoError(t, err)
		var got []fse.Prescription
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, want, got)
	})
}

func TestWithRefresh(t *testing.T) {
	t.Run("no scope is not forced", func(t *testing.T) {
		assert.False(t, forced(context.Background(), testNRE))
	})

	t.Run("All forces everything", func(t *testing.T) {
		ctx := WithRefresh(context.Background(), Scope{All: true})
		assert.True(t, forced(ctx, ""))
		assert.True(t, forced(ctx, testNRE))
	})

	t.Run("NRE scope forces only that NRE", func(t *testing.T) {
		ctx := WithRefresh(context.Background(), Scope{NREs: []string{testNRE}})
		assert.True(t, forced(ctx, testNRE))
		assert.False(t, forced(ctx, ""))
	})
}

func TestFetchedAt(t *testing.T) {
	t.Run("maps subject to time", func(t *testing.T) {
		st := &MockStore{LatestBySourceFunc: func(ctx context.Context, source string) ([]store.Record, error) {
			return []store.Record{
				{Source: source, Subject: testNRE, FetchedAt: testNow},
				{Source: source, Subject: "0900A0000000002", FetchedAt: testNow.Add(-time.Hour)},
			}, nil
		}}
		got, err := FetchedAt(context.Background(), st, SourceCUPAppointments)
		require.NoError(t, err)
		assert.Equal(t, map[string]time.Time{testNRE: testNow, "0900A0000000002": testNow.Add(-time.Hour)}, got)
		assert.Equal(t, SourceCUPAppointments, st.LatestBySourceCalls()[0].Source)
	})

	t.Run("store error propagates", func(t *testing.T) {
		st := &MockStore{LatestBySourceFunc: func(ctx context.Context, source string) ([]store.Record, error) {
			return nil, errors.New("boom")
		}}
		_, err := FetchedAt(context.Background(), st, SourceCUPAppointments)
		require.Error(t, err)
	})
}

func fakeListing() referti.Listing {
	return referti.Listing{
		Reports: []referti.Report{{
			Ref:       referti.Ref{Kind: referti.KindLab, AbstractID: "ABS1", DetailID: "DET1", RepCode: "REP1"},
			Date:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
			Provider:  "ASL",
			Unread:    true,
		}},
		Failures: []referti.Failure{{Kind: referti.KindRadiology, Err: errors.New("upstream 502")}},
	}
}

func TestReportsList(t *testing.T) {
	want := fakeListing()
	newReports := func(src ReportsSource, st Store) *Reports {
		r := NewReports(src, st, DefaultPolicy())
		r.now = func() time.Time { return testNow }
		return r
	}
	missStore := func() *MockStore {
		return &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return store.Record{}, store.ErrNotFound
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
	}

	t.Run("fresh hit skips source and keeps failures", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow.Add(-time.Minute), toStored(want)), nil
		}}
		src := &MockReportsSource{}
		got, err := newReports(src, st).List(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want.Reports, got.Reports)
		require.Len(t, got.Failures, 1)
		assert.Equal(t, referti.KindRadiology, got.Failures[0].Kind)
		assert.EqualError(t, got.Failures[0].Err, "upstream 502")
		assert.Empty(t, src.ListCalls())
		assert.Equal(t, SourceFSEReports, st.LatestCalls()[0].Source)
	})

	t.Run("miss stores listing with failures as kind and message", func(t *testing.T) {
		st := missStore()
		src := &MockReportsSource{ListFunc: func(ctx context.Context) (referti.Listing, error) { return want, nil }}
		got, err := newReports(src, st).List(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want.Reports, got.Reports)
		require.Len(t, st.PutCalls(), 1)
		assert.JSONEq(t, `{"kind":"ris","message":"upstream 502"}`, string(jsonFailure(t, st.PutCalls()[0].Rec.Payload)))
	})

	t.Run("stale record refetched and stored even with failures", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return record(t, source, subject, testNow.Add(-2*time.Hour), toStored(referti.Listing{})), nil
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockReportsSource{ListFunc: func(ctx context.Context) (referti.Listing, error) { return want, nil }}
		got, err := newReports(src, st).List(context.Background())
		require.NoError(t, err)
		assert.Len(t, got.Reports, 1)
		assert.Len(t, st.PutCalls(), 1)
	})

	t.Run("forced by All", func(t *testing.T) {
		st := &MockStore{
			LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
				return record(t, source, subject, testNow, toStored(referti.Listing{})), nil
			},
			PutFunc: func(ctx context.Context, rec store.Record) error { return nil },
		}
		src := &MockReportsSource{ListFunc: func(ctx context.Context) (referti.Listing, error) { return want, nil }}
		got, err := newReports(src, st).List(WithRefresh(context.Background(), Scope{All: true}))
		require.NoError(t, err)
		assert.Len(t, got.Reports, 1)
		assert.Len(t, src.ListCalls(), 1)
	})

	t.Run("NRE scope does not force", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow, toStored(want)), nil
		}}
		src := &MockReportsSource{}
		_, err := newReports(src, st).List(WithRefresh(context.Background(), Scope{NREs: []string{testNRE}}))
		require.NoError(t, err)
		assert.Empty(t, src.ListCalls())
	})

	t.Run("source error with stale serves stale", func(t *testing.T) {
		st := &MockStore{LatestFunc: func(ctx context.Context, source, subject string) (store.Record, error) {
			return record(t, source, subject, testNow.Add(-2*time.Hour), toStored(want)), nil
		}}
		src := &MockReportsSource{ListFunc: func(ctx context.Context) (referti.Listing, error) {
			return referti.Listing{}, errors.New("boom")
		}}
		got, err := newReports(src, st).List(context.Background())
		require.NoError(t, err)
		assert.Equal(t, want.Reports, got.Reports)
	})

	t.Run("source error without record keeps error", func(t *testing.T) {
		src := &MockReportsSource{ListFunc: func(ctx context.Context) (referti.Listing, error) {
			return referti.Listing{}, session.ErrNotLoggedIn
		}}
		_, err := newReports(src, missStore()).List(context.Background())
		require.ErrorIs(t, err, session.ErrNotLoggedIn)
	})
}

func jsonFailure(t *testing.T, payload []byte) []byte {
	t.Helper()
	var stored struct {
		Failures []json.RawMessage `json:"failures"`
	}
	require.NoError(t, json.Unmarshal(payload, &stored))
	require.Len(t, stored.Failures, 1)
	return stored.Failures[0]
}
