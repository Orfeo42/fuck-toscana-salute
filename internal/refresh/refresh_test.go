package refresh

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/reports"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

const wait = 2 * time.Second

func startRun(t *testing.T, r *Refresher) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func recv(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(wait):
		t.Fatal("timeout")
	}
}

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	require.Eventually(t, cond, wait, time.Millisecond)
}

func TestClassify(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Equal(t, IssueNone, Classify(nil))
	})
	t.Run("session expired", func(t *testing.T) {
		assert.Equal(t, IssueLoginRequired, Classify(fmt.Errorf("x: %w", session.ErrSessionExpired)))
	})
	t.Run("not logged in", func(t *testing.T) {
		assert.Equal(t, IssueLoginRequired, Classify(session.ErrNotLoggedIn))
	})
	t.Run("gestione not ready", func(t *testing.T) {
		assert.Equal(t, IssueGestioneRequired, Classify(fmt.Errorf("x: %w", cup.ErrGestioneNotReady)))
	})
	t.Run("upstream status", func(t *testing.T) {
		assert.Equal(t, IssueUnavailable, Classify(&upstream.StatusError{Status: 503}))
	})
	t.Run("other", func(t *testing.T) {
		assert.Equal(t, IssueUnavailable, Classify(errors.New("boom")))
	})
}

func TestStartEmptyScope(t *testing.T) {
	r := New(&MockAgenda{}, &MockReports{})
	require.ErrorIs(t, r.Start(Scope{}), ErrEmptyScope)
}

func TestKickSingleFlight(t *testing.T) {
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	ag := &MockAgenda{AppointmentsFunc: func(ctx context.Context) (agenda.Result, error) {
		entered <- struct{}{}
		<-release
		return agenda.Result{}, nil
	}}
	r := New(ag, &MockReports{})
	startRun(t, r)

	t.Run("running job is not queued again", func(t *testing.T) {
		r.Kick(JobVisits)
		recv(t, entered)
		r.Kick(JobVisits)
		r.Kick(JobVisits)
		assert.Equal(t, []Job{JobVisits}, r.Status().Jobs)
		assert.True(t, r.Status().Active)
		close(release)
		eventually(t, func() bool { return !r.Status().Active })
		assert.Len(t, ag.AppointmentsCalls(), 1)
	})
}

func TestStartMergesQueuedScopes(t *testing.T) {
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	ag := &MockAgenda{
		AppointmentsFunc: func(ctx context.Context) (agenda.Result, error) {
			entered <- struct{}{}
			<-release
			return agenda.Result{}, nil
		},
		PrescriptionsFunc: func(ctx context.Context) ([]agenda.Row, error) { return nil, nil },
	}
	rep := &MockReports{ListFunc: func(ctx context.Context) (reports.Listing, error) { return reports.Listing{}, nil }}
	r := New(ag, rep)
	startRun(t, r)

	t.Run("duplicates collapse", func(t *testing.T) {
		r.Kick(JobVisits)
		recv(t, entered)
		require.NoError(t, r.Start(Scope{Lists: true}))
		require.NoError(t, r.Start(Scope{Lists: true}))
		require.NoError(t, r.Start(Scope{Reports: true}))
		r.Kick(JobLists)
		assert.Equal(t, []Job{JobVisits, JobLists, JobReports}, r.Status().Jobs)
		close(release)
		eventually(t, func() bool { return !r.Status().Active })
		assert.Len(t, ag.PrescriptionsCalls(), 1)
		assert.Len(t, rep.ListCalls(), 1)
	})
}

func TestNRELaneRunsDuringBulk(t *testing.T) {
	bulkEntered := make(chan struct{}, 1)
	bulkRelease := make(chan struct{})
	ag := &MockAgenda{
		AppointmentsFunc: func(ctx context.Context) (agenda.Result, error) {
			bulkEntered <- struct{}{}
			<-bulkRelease
			return agenda.Result{}, nil
		},
		RefreshNREsFunc: func(ctx context.Context, nres []string) error { return nil },
	}
	r := New(ag, &MockReports{})
	startRun(t, r)

	t.Run("nre refresh completes while bulk blocked", func(t *testing.T) {
		r.Kick(JobVisits)
		recv(t, bulkEntered)
		require.NoError(t, r.Start(Scope{NREs: []string{"A", "B", "A"}}))
		eventually(t, func() bool { return len(ag.RefreshNREsCalls()) == 1 })
		eventually(t, func() bool { return len(r.Status().NREs) == 0 })
		assert.Equal(t, []string{"A", "B"}, ag.RefreshNREsCalls()[0].Nres)
		assert.Equal(t, []Job{JobVisits}, r.Status().Jobs)
		close(bulkRelease)
		eventually(t, func() bool { return !r.Status().Active })
	})
}

func TestBackoffOnlyOnUnavailable(t *testing.T) {
	now := time.Unix(1000, 0)
	var errToReturn error
	ag := &MockAgenda{PrescriptionsFunc: func(ctx context.Context) ([]agenda.Row, error) { return nil, errToReturn }}
	r := New(ag, &MockReports{})
	r.now = func() time.Time { return now }
	startRun(t, r)

	runOnce := func(calls int) {
		r.Kick(JobLists)
		eventually(t, func() bool { return len(ag.PrescriptionsCalls()) == calls && !r.Status().Active })
	}

	t.Run("unavailable backs off", func(t *testing.T) {
		errToReturn = errors.New("boom")
		runOnce(1)
		r.Kick(JobLists)
		assert.Empty(t, r.Status().Jobs)
		assert.Equal(t, IssueUnavailable, r.Status().Issues.Lists)
	})
	t.Run("backoff expires", func(t *testing.T) {
		now = now.Add(61 * time.Second)
		errToReturn = session.ErrNotLoggedIn
		runOnce(2)
		assert.Equal(t, IssueLoginRequired, r.Status().Issues.Lists)
	})
	t.Run("login failure is not backed off", func(t *testing.T) {
		errToReturn = fmt.Errorf("x: %w", cup.ErrGestioneNotReady)
		runOnce(3)
		assert.Equal(t, IssueGestioneRequired, r.Status().Issues.Lists)
	})
}

func TestStatusReturnsCopies(t *testing.T) {
	ag := &MockAgenda{AppointmentsFunc: func(ctx context.Context) (agenda.Result, error) {
		return agenda.Result{Failed: []agenda.Failure{{NRE: "N1", Err: session.ErrSessionExpired}}}, nil
	}}
	r := New(ag, &MockReports{})
	startRun(t, r)

	t.Run("mutating status leaves internal state", func(t *testing.T) {
		r.Kick(JobVisits)
		eventually(t, func() bool { return len(r.Status().Failed) == 1 })
		first := r.Status()
		assert.Equal(t, IssueLoginRequired, first.Failed["N1"])
		first.Failed["N1"] = IssueNone
		first.Failed["N2"] = IssueNone
		assert.Equal(t, map[string]Issue{"N1": IssueLoginRequired}, r.Status().Failed)
	})
	t.Run("queued slices are copies", func(t *testing.T) {
		r2 := New(&MockAgenda{}, &MockReports{})
		require.NoError(t, r2.Start(Scope{NREs: []string{"X"}, Lists: true}))
		st := r2.Status()
		st.NREs[0] = "changed"
		st.Jobs[0] = JobVisits
		assert.Equal(t, []string{"X"}, r2.Status().NREs)
		assert.Equal(t, []Job{JobLists}, r2.Status().Jobs)
	})
}

func TestNREFailureRecorded(t *testing.T) {
	var fail error = &upstream.StatusError{Status: 500}
	ag := &MockAgenda{RefreshNREsFunc: func(ctx context.Context, nres []string) error { return fail }}
	r := New(ag, &MockReports{})
	startRun(t, r)

	t.Run("failure then success clears", func(t *testing.T) {
		require.NoError(t, r.Start(Scope{NREs: []string{"N1"}}))
		eventually(t, func() bool { return r.Status().Failed["N1"] == IssueUnavailable })
		fail = nil
		require.NoError(t, r.Start(Scope{NREs: []string{"N1"}}))
		eventually(t, func() bool { return len(r.Status().Failed) == 0 })
	})
}

func TestRunStopsOnCancel(t *testing.T) {
	r := New(&MockAgenda{}, &MockReports{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	t.Run("run returns", func(t *testing.T) {
		cancel()
		select {
		case <-done:
		case <-time.After(wait):
			t.Fatal("Run did not return")
		}
	})
}

func TestForcedScopeReachesCache(t *testing.T) {
	ctxs := make(chan context.Context, 1)
	ag := &MockAgenda{PrescriptionsFunc: func(ctx context.Context) ([]agenda.Row, error) {
		ctxs <- ctx
		return nil, nil
	}}
	r := New(ag, &MockReports{})
	startRun(t, r)

	t.Run("start lists runs prescriptions", func(t *testing.T) {
		require.NoError(t, r.Start(Scope{Lists: true}))
		select {
		case ctx := <-ctxs:
			assert.NoError(t, ctx.Err())
		case <-time.After(wait):
			t.Fatal("timeout")
		}
	})
}
