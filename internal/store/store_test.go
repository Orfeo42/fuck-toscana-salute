package store

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	fakeNRE       = "0900A0000000001"
	payloadMarker = "PAYLOAD_MARKER_XYZ"
)

func testKey(b byte) []byte {
	return bytes.Repeat([]byte{b}, 32)
}

var (
	testDSN string
	testDB  *sqlx.DB
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, "postgres:18.6-alpine",
		postgres.WithDatabase("fts"),
		postgres.WithUsername("fts"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres container:", err)
		return 1
	}
	defer func() { _ = ctr.Terminate(ctx) }()
	testDSN, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "dsn:", err)
		return 1
	}
	testDB, err = sqlx.Open("pgx", testDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open test db:", err)
		return 1
	}
	defer func() {
		if err := testDB.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "close test db:", err)
		}
	}()
	return m.Run()
}

func openTest(t *testing.T) *Store {
	t.Helper()
	_, err := testDB.ExecContext(context.Background(), "DROP TABLE IF EXISTS responses")
	require.NoError(t, err)
	s, err := Open(context.Background(), testDSN, testKey(1))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func countRows(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, testDB.GetContext(context.Background(), &n, "SELECT COUNT(*) FROM responses"))
	return n
}

func rec(subject string, at time.Time, payload string) Record {
	return Record{Source: "cup.verdict", Subject: subject, FetchedAt: at, Status: 200, Payload: []byte(payload)}
}

func TestOpen(t *testing.T) {
	ctx := context.Background()

	t.Run("idempotent twice", func(t *testing.T) {
		s1 := openTest(t)
		s2, err := Open(ctx, testDSN, testKey(1))
		require.NoError(t, err)
		require.NoError(t, s2.Close())
		require.NoError(t, s1.Close())
	})

	t.Run("reopen with same key reads data", func(t *testing.T) {
		s1 := openTest(t)
		at := time.UnixMilli(1_700_000_000_000)
		require.NoError(t, s1.Put(ctx, rec(fakeNRE, at, `{"a":1}`)))
		require.NoError(t, s1.Close())
		s2, err := Open(ctx, testDSN, testKey(1))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, s2.Close()) })
		got, err := s2.Latest(ctx, "cup.verdict", fakeNRE)
		require.NoError(t, err)
		assert.JSONEq(t, `{"a":1}`, string(got.Payload))
	})

	t.Run("wrong key fails to decrypt", func(t *testing.T) {
		s1 := openTest(t)
		require.NoError(t, s1.Put(ctx, rec(fakeNRE, time.Now(), "x")))
		require.NoError(t, s1.Close())
		_, err := Open(ctx, testDSN, testKey(2))
		require.ErrorIs(t, err, ErrDecrypt)
	})

	t.Run("bad key length", func(t *testing.T) {
		_, err := Open(ctx, testDSN, []byte("short"))
		require.ErrorIs(t, err, ErrKey)
	})

	t.Run("unreachable database", func(t *testing.T) {
		_, err := Open(ctx, "postgres://fts:x@127.0.0.1:1/fts?sslmode=disable&connect_timeout=2", testKey(1))
		require.ErrorIs(t, err, ErrConnect)
	})
}

func TestPut(t *testing.T) {
	ctx := context.Background()

	t.Run("appends history", func(t *testing.T) {
		s := openTest(t)
		at := time.UnixMilli(1_700_000_000_000)
		require.NoError(t, s.Put(ctx, rec(fakeNRE, at, "1")))
		require.NoError(t, s.Put(ctx, rec(fakeNRE, at.Add(time.Minute), "2")))
		require.NoError(t, s.Put(ctx, rec(fakeNRE, at.Add(2*time.Minute), "3")))
		assert.Equal(t, 3, countRows(t))
	})

	t.Run("no clear subject or payload stored", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.Put(ctx, rec(fakeNRE, time.Now(), payloadMarker)))
		var r row
		require.NoError(t, testDB.GetContext(ctx, &r, anyRowQuery))
		assert.NotEqual(t, fakeNRE, r.SubjectMAC)
		assert.NotContains(t, r.SubjectMAC, fakeNRE)
		assert.False(t, bytes.Contains(r.SubjectEnc, []byte(fakeNRE)))
		assert.False(t, bytes.Contains(r.PayloadEnc, []byte(payloadMarker)))
	})
}

func TestLatest(t *testing.T) {
	ctx := context.Background()

	t.Run("round trip", func(t *testing.T) {
		s := openTest(t)
		at := time.UnixMilli(1_700_000_000_123)
		in := Record{Source: "cup.verdict", Subject: fakeNRE, FetchedAt: at, Status: 404, Payload: []byte(`{"k":"v"}`)}
		require.NoError(t, s.Put(ctx, in))
		got, err := s.Latest(ctx, "cup.verdict", fakeNRE)
		require.NoError(t, err)
		assert.Equal(t, in.Source, got.Source)
		assert.Equal(t, in.Subject, got.Subject)
		assert.Equal(t, 404, got.Status)
		assert.True(t, at.Equal(got.FetchedAt))
		assert.Equal(t, in.Payload, got.Payload)
	})

	t.Run("newest of several", func(t *testing.T) {
		s := openTest(t)
		at := time.UnixMilli(1_700_000_000_000)
		require.NoError(t, s.Put(ctx, rec(fakeNRE, at.Add(time.Minute), "new")))
		require.NoError(t, s.Put(ctx, rec(fakeNRE, at, "old")))
		got, err := s.Latest(ctx, "cup.verdict", fakeNRE)
		require.NoError(t, err)
		assert.Equal(t, "new", string(got.Payload))
	})

	t.Run("not found", func(t *testing.T) {
		s := openTest(t)
		_, err := s.Latest(ctx, "cup.verdict", fakeNRE)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("empty subject", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.Put(ctx, rec("", time.Now(), "list")))
		got, err := s.Latest(ctx, "cup.verdict", "")
		require.NoError(t, err)
		assert.Equal(t, "list", string(got.Payload))
	})

	t.Run("row AAD swap detected", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.Put(ctx, rec(fakeNRE, time.Now(), "a")))
		require.NoError(t, s.Put(ctx, rec("0900A0000000002", time.Now(), "b")))
		_, err := s.db.ExecContext(ctx, `UPDATE responses SET payload_enc = (SELECT payload_enc FROM responses WHERE id = 2) WHERE id = 1`)
		require.NoError(t, err)
		_, err = s.Latest(ctx, "cup.verdict", fakeNRE)
		require.ErrorIs(t, err, ErrDecrypt)
	})
}

func TestLatestBySource(t *testing.T) {
	ctx := context.Background()

	t.Run("one per subject newest each", func(t *testing.T) {
		s := openTest(t)
		at := time.UnixMilli(1_700_000_000_000)
		require.NoError(t, s.Put(ctx, rec("A", at, "a1")))
		require.NoError(t, s.Put(ctx, rec("A", at.Add(time.Minute), "a2")))
		require.NoError(t, s.Put(ctx, rec("B", at, "b1")))
		require.NoError(t, s.Put(ctx, Record{Source: "other", Subject: "C", FetchedAt: at, Status: 200, Payload: []byte("c")}))
		got, err := s.LatestBySource(ctx, "cup.verdict")
		require.NoError(t, err)
		require.Len(t, got, 2)
		payloads := map[string]string{}
		for _, r := range got {
			payloads[r.Subject] = string(r.Payload)
		}
		assert.Equal(t, map[string]string{"A": "a2", "B": "b1"}, payloads)
	})

	t.Run("empty source returns none", func(t *testing.T) {
		s := openTest(t)
		got, err := s.LatestBySource(ctx, "cup.verdict")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestClose(t *testing.T) {
	t.Run("closes", func(t *testing.T) {
		s := openTest(t)
		require.NoError(t, s.Close())
	})
}
