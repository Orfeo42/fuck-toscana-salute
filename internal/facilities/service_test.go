package facilities

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	testDB  *sqlx.DB
	testNow = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
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
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "dsn:", err)
		return 1
	}
	testDB, err = sqlx.Open("pgx", dsn)
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

type response struct {
	status int
	body   []byte
}

type fakeSite struct {
	t       *testing.T
	files   map[string]response
	server  *httptest.Server
	baseURL string
}

func newFakeSite(t *testing.T) *fakeSite {
	t.Helper()
	site := &fakeSite{t: t, files: make(map[string]response)}
	site.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, ok := site.files[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if resp.status != 0 {
			w.WriteHeader(resp.status)
		}
		_, err := w.Write(resp.body)
		assert.NoError(t, err)
	}))
	t.Cleanup(site.server.Close)
	site.baseURL = site.server.URL
	site.serveAll()
	return site
}

func (f *fakeSite) serveAll() {
	datasets := map[string][]byte{
		ministrySpecs[0].slug: []byte(apparecchiatureCSV),
		ministrySpecs[1].slug: latin1(f.t, ricoveroCSV),
		ministrySpecs[2].slug: latin1(f.t, caseDiCuraCSV),
		ministrySpecs[3].slug: latin1(f.t, aziendeCSV),
		ministrySpecs[4].slug: latin1(f.t, consultoriCSV),
	}
	for slug, body := range datasets {
		f.files["/page-data/it/dataset/"+slug+"/page-data.json"] = response{body: pageDataJSON(f.t, "/sites/default/files/2024-01/"+slug+".csv")}
		f.files["/sites/default/files/2024-01/"+slug+".csv"] = response{body: body}
	}
	f.files["/ckan"] = response{body: ckanJSON(f.t, f.baseURL+"/rt/territorialifeature.csv")}
	f.files["/rt/territorialifeature.csv"] = response{body: latin1(f.t, regioneCSV)}
}

func pageDataJSON(t *testing.T, fileURL string) []byte {
	t.Helper()
	doc := map[string]any{"result": map[string]any{"data": map[string]any{"allNodeDataset": map[string]any{"nodes": []any{
		map[string]any{"relationships": map[string]any{"field_listafile": []any{
			map[string]any{"url": "/sites/default/files/readme.pdf"},
			map[string]any{"url": fileURL},
		}}},
	}}}}}
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	return out
}

func ckanJSON(t *testing.T, fileURL string) []byte {
	t.Helper()
	doc := map[string]any{"result": map[string]any{"resources": []any{
		map[string]any{"url": "https://example.invalid/other.json"},
		map[string]any{"url": fileURL},
	}}}
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	return out
}

func newTestService(t *testing.T, site *fakeSite) *Service {
	t.Helper()
	_, err := testDB.ExecContext(context.Background(), "DROP TABLE IF EXISTS facilities, facilities_meta")
	require.NoError(t, err)
	s, err := New(context.Background(), testDB, site.server.Client())
	require.NoError(t, err)
	s.ministryBase = site.baseURL
	s.ckanURL = site.baseURL + "/ckan"
	return s
}

func countSource(t *testing.T, source string) int {
	t.Helper()
	query, args, err := sqlx.Named("SELECT COUNT(*) FROM facilities WHERE source = :source", sourceArg{Source: source})
	require.NoError(t, err)
	var n int
	require.NoError(t, testDB.GetContext(context.Background(), &n, testDB.Rebind(query), args...))
	return n
}

func TestNew(t *testing.T) {
	t.Run("is idempotent", func(t *testing.T) {
		site := newFakeSite(t)
		newTestService(t, site)
		_, err := New(context.Background(), testDB, site.server.Client())
		require.NoError(t, err)
	})
}

func TestResolveFileURLs(t *testing.T) {
	ctx := context.Background()

	t.Run("ministry picks the csv and prefixes a relative url", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		got, err := s.ministryFileURL(ctx, ministrySpecs[1].slug)
		require.NoError(t, err)
		assert.Equal(t, site.baseURL+"/sites/default/files/2024-01/"+ministrySpecs[1].slug+".csv", got)
	})

	t.Run("ministry keeps an absolute url", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		site.files["/page-data/it/dataset/x/page-data.json"] = response{body: pageDataJSON(t, "https://files.example.test/a.CSV")}
		got, err := s.ministryFileURL(ctx, "x")
		require.NoError(t, err)
		assert.Equal(t, "https://files.example.test/a.CSV", got)
	})

	t.Run("ministry without csv fails", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		site.files["/page-data/it/dataset/x/page-data.json"] = response{body: pageDataJSON(t, "/a.pdf")}
		_, err := s.ministryFileURL(ctx, "x")
		require.ErrorIs(t, err, ErrNoCSV)
	})

	t.Run("ministry without nodes fails", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		site.files["/page-data/it/dataset/x/page-data.json"] = response{body: []byte(`{"result":{"data":{"allNodeDataset":{"nodes":[]}}}}`)}
		_, err := s.ministryFileURL(ctx, "x")
		require.ErrorIs(t, err, ErrNoCSV)
	})

	t.Run("ministry page not found fails", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		_, err := s.ministryFileURL(ctx, "missing")
		require.ErrorIs(t, err, ErrStatus)
	})

	t.Run("regione picks the territorialifeature resource", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		got, err := s.regioneFileURL(ctx)
		require.NoError(t, err)
		assert.Equal(t, site.baseURL+"/rt/territorialifeature.csv", got)
	})

	t.Run("regione without the resource fails", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		site.files["/ckan"] = response{body: []byte(`{"result":{"resources":[{"url":"https://example.invalid/x.json"}]}}`)}
		_, err := s.regioneFileURL(ctx)
		require.ErrorIs(t, err, ErrNoResource)
	})
}

func TestFetch(t *testing.T) {
	ctx := context.Background()

	t.Run("download over the cap fails", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		site.files["/big"] = response{body: []byte(strings.Repeat("a", maxDownloadBytes+1))}
		_, err := s.fetch(ctx, site.baseURL+"/big")
		require.ErrorIs(t, err, ErrTooLarge)
	})

	t.Run("non 200 status fails", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		site.files["/err"] = response{status: http.StatusInternalServerError}
		_, err := s.fetch(ctx, site.baseURL+"/err")
		require.ErrorIs(t, err, ErrStatus)
	})
}

func TestRefresh(t *testing.T) {
	ctx := context.Background()

	t.Run("stores both sources and search finds them", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		require.NoError(t, s.Refresh(ctx))
		assert.Equal(t, 6, countSource(t, SourceMinistero))
		assert.Equal(t, 2, countSource(t, SourceRegione))
		got, err := s.Search(ctx, "careggi")
		require.NoError(t, err)
		assert.Equal(t, []Facility{{Code: "090101", Name: "AOU CAREGGI", Comune: "FIRENZE", Provincia: "FI", Address: "VIALE MORGAGNI 85", Source: SourceMinistero}}, got)
	})

	t.Run("a failing regione source keeps its old rows and still updates ministero", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		require.NoError(t, s.Refresh(ctx))
		site.files["/rt/territorialifeature.csv"] = response{status: http.StatusInternalServerError}
		site.files["/sites/default/files/2024-01/"+ministrySpecs[2].slug+".csv"] = response{body: latin1(t, "Anno;Codice struttura;Denominazione struttura;Comune struttura;Sigla provincia struttura;Codice regione\n2022;090501;CASA NUOVA;LIVORNO;LI;090\n")}

		err := s.Refresh(ctx)

		require.ErrorIs(t, err, ErrStatus)
		assert.Equal(t, 2, countSource(t, SourceRegione))
		assert.Equal(t, 6, countSource(t, SourceMinistero))
		got, err := s.Search(ctx, "casa nuova")
		require.NoError(t, err)
		require.Len(t, got, 1)
		got, err = s.Search(ctx, "villa serena")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("a failing ministry file keeps all old ministero rows and updates regione", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		require.NoError(t, s.Refresh(ctx))
		site.files["/sites/default/files/2024-01/"+ministrySpecs[4].slug+".csv"] = response{status: http.StatusBadGateway}
		site.files["/rt/territorialifeature.csv"] = response{body: latin1(t, "Denominazione Struttura,Codice Struttura,Data Chiusura\nSOLO UNO,R100,31/12/9999\n")}

		err := s.Refresh(ctx)

		require.ErrorIs(t, err, ErrStatus)
		assert.Equal(t, 6, countSource(t, SourceMinistero))
		assert.Equal(t, 1, countSource(t, SourceRegione))
	})
}

func TestStale(t *testing.T) {
	ctx := context.Background()

	t.Run("empty table is stale", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		stale, err := s.Stale(ctx)
		require.NoError(t, err)
		assert.True(t, stale)
	})

	t.Run("fresh after a refresh", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		require.NoError(t, s.Refresh(ctx))
		stale, err := s.Stale(ctx)
		require.NoError(t, err)
		assert.False(t, stale)
	})

	t.Run("older than 30 days is stale", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		require.NoError(t, s.Refresh(ctx))
		_, err := testDB.ExecContext(ctx, "UPDATE facilities_meta SET refreshed_at = now() - interval '31 days' WHERE source = 'regione'")
		require.NoError(t, err)
		stale, err := s.Stale(ctx)
		require.NoError(t, err)
		assert.True(t, stale)
	})
}

func TestRefreshIfStale(t *testing.T) {
	ctx := context.Background()

	t.Run("refreshes when stale", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		s.refreshIfStale(ctx)
		assert.Equal(t, 2, countSource(t, SourceRegione))
	})

	t.Run("keeps data when fresh", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		require.NoError(t, s.Refresh(ctx))
		site.files["/ckan"] = response{status: http.StatusInternalServerError}
		s.refreshIfStale(ctx)
		assert.Equal(t, 2, countSource(t, SourceRegione))
	})

	t.Run("failed refresh keeps the table untouched", func(t *testing.T) {
		site := newFakeSite(t)
		s := newTestService(t, site)
		site.files["/ckan"] = response{status: http.StatusInternalServerError}
		s.refreshIfStale(ctx)
		assert.Equal(t, 0, countSource(t, SourceRegione))
		assert.Equal(t, 6, countSource(t, SourceMinistero))
	})
}

func seed(t *testing.T, s *Service, source string, list []Facility) {
	t.Helper()
	require.NoError(t, s.replaceSource(context.Background(), source, list, testNow))
}

func names(list []Facility) []string {
	out := make([]string, 0, len(list))
	for _, f := range list {
		out = append(out, f.Name)
	}
	return out
}

func TestSearch(t *testing.T) {
	ctx := context.Background()

	t.Run("every token must match", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		seed(t, s, SourceMinistero, []Facility{
			{Code: "1", Name: "Ospedale Santa Chiara", Comune: "Pisa"},
			{Code: "2", Name: "Ospedale Santa Chiara", Comune: "Siena"},
		})
		got, err := s.Search(ctx, "chiara pisa")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "Pisa", got[0].Comune)
	})

	t.Run("ignores accents and case in both directions", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		seed(t, s, SourceMinistero, []Facility{{Code: "1", Name: "Clinica Città Nuova", Comune: "Prato"}})
		got, err := s.Search(ctx, "CITTA")
		require.NoError(t, err)
		assert.Len(t, got, 1)
		got, err = s.Search(ctx, "città")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("matches the comune", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		seed(t, s, SourceRegione, []Facility{{Code: "1", Name: "Studio Neri", Comune: "Lucca"}})
		got, err := s.Search(ctx, "lucca")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("orders ministero first, then name prefix, then name", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		seed(t, s, SourceMinistero, []Facility{
			{Code: "1", Name: "Zeta Centro Medico", Comune: "Pisa"},
			{Code: "2", Name: "Centro Alfa", Comune: "Pisa"},
			{Code: "3", Name: "Alfa Centro", Comune: "Pisa"},
		})
		seed(t, s, SourceRegione, []Facility{{Code: "1", Name: "Centro Aaa", Comune: "Pisa"}})
		got, err := s.Search(ctx, "centro")
		require.NoError(t, err)
		assert.Equal(t, []string{"Centro Alfa", "Alfa Centro", "Zeta Centro Medico", "Centro Aaa"}, names(got))
	})

	t.Run("limits to 20 results", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		list := make([]Facility, 0, 25)
		for i := range 25 {
			list = append(list, Facility{Code: fmt.Sprintf("c%02d", i), Name: fmt.Sprintf("Poliambulatorio %02d", i), Comune: "Pisa"})
		}
		seed(t, s, SourceMinistero, list)
		got, err := s.Search(ctx, "poliambulatorio")
		require.NoError(t, err)
		assert.Len(t, got, 20)
	})

	t.Run("escapes like wildcards", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		seed(t, s, SourceMinistero, []Facility{
			{Code: "1", Name: "Studio A_B", Comune: "Pisa"},
			{Code: "2", Name: "Studio AXB", Comune: "Pisa"},
			{Code: "3", Name: "Studio 100%", Comune: "Pisa"},
		})
		got, err := s.Search(ctx, "a_b")
		require.NoError(t, err)
		assert.Equal(t, []string{"Studio A_B"}, names(got))
		got, err = s.Search(ctx, "100%")
		require.NoError(t, err)
		assert.Equal(t, []string{"Studio 100%"}, names(got))
	})

	t.Run("blank query returns nothing", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		seed(t, s, SourceMinistero, []Facility{{Code: "1", Name: "Studio", Comune: "Pisa"}})
		got, err := s.Search(ctx, "   ")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestReplaceSource(t *testing.T) {
	ctx := context.Background()

	t.Run("replaces only its own source", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		seed(t, s, SourceMinistero, []Facility{{Code: "1", Name: "Uno", Comune: "Pisa"}})
		seed(t, s, SourceRegione, []Facility{{Code: "1", Name: "Due", Comune: "Pisa"}})
		seed(t, s, SourceMinistero, []Facility{{Code: "9", Name: "Tre", Comune: "Pisa"}})
		assert.Equal(t, 1, countSource(t, SourceMinistero))
		assert.Equal(t, 1, countSource(t, SourceRegione))
	})

	t.Run("duplicate key rolls back and keeps old rows", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		seed(t, s, SourceMinistero, []Facility{{Code: "1", Name: "Uno", Comune: "Pisa"}})
		err := s.replaceSource(ctx, SourceMinistero, []Facility{{Code: "2", Name: "A"}, {Code: "2", Name: "B"}}, testNow)
		require.Error(t, err)
		got, err := s.Search(ctx, "uno")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("inserts more rows than one chunk", func(t *testing.T) {
		s := newTestService(t, newFakeSite(t))
		list := make([]Facility, 0, insertChunk+10)
		for i := range insertChunk + 10 {
			list = append(list, Facility{Code: fmt.Sprintf("c%d", i), Name: "Struttura", Comune: "Pisa"})
		}
		seed(t, s, SourceMinistero, list)
		assert.Equal(t, insertChunk+10, countSource(t, SourceMinistero))
	})
}
