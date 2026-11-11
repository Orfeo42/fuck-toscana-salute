package referti

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

type recordedRequest struct {
	Path string
	Auth string
	Body string
}

type recorder struct {
	mu   sync.Mutex
	reqs []recordedRequest
}

func (r *recorder) add(req recordedRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, req)
}

func (r *recorder) find(path string) (recordedRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, req := range r.reqs {
		if req.Path == path {
			return req, true
		}
	}
	return recordedRequest{}, false
}

var forbiddenFragments = []string{"updateonlyconsultationstatusofact", "oscura", "consen", "deleg"}

const pdfBase64 = "JVBERi0xLjQK"

func newTokens(token string, err error) *MockTokenSource {
	return &MockTokenSource{
		AccessTokenFunc: func(ctx context.Context, portal session.Portal) (string, error) {
			return token, err
		},
	}
}

func newTestClient(t *testing.T, tokens TokenSource, rec *recorder, respond http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		req := recordedRequest{Path: r.URL.Path, Auth: r.Header.Get("Authorization"), Body: string(body)}
		lower := strings.ToLower(r.URL.Path)
		for _, forbidden := range forbiddenFragments {
			if strings.Contains(lower, forbidden) {
				t.Errorf("forbidden path requested: %s", r.URL.Path)
			}
		}
		rec.add(req)
		respond(w, r)
	}))
	t.Cleanup(srv.Close)
	c := New(upstream.New(srv.Client()), tokens)
	for _, kind := range allKinds {
		c.bases[kind] = srv.URL + "/" + string(kind)
	}
	return c
}

func write(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	_, err := w.Write([]byte(body))
	require.NoError(t, err)
}

func TestKindLabel(t *testing.T) {
	t.Run("italian labels", func(t *testing.T) {
		assert.Equal(t, "Laboratorio analisi", KindLab.Label())
		assert.Equal(t, "Trasfusionale", KindTransfusion.Label())
		assert.Equal(t, "Radiologia", KindRadiology.Label())
		assert.Equal(t, "Lettere di dimissione", KindDischarge.Label())
		assert.Equal(t, "Pronto soccorso", KindER.Label())
		assert.Equal(t, "Specialistica ambulatoriale", KindSpecialist.Label())
		assert.Equal(t, "Anatomia patologica", KindPathology.Label())
	})

	t.Run("unknown kind falls back to its name", func(t *testing.T) {
		assert.Equal(t, "zzz", Kind("zzz").Label())
	})
}

func TestParseKind(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := ParseKind("labtrasf")

		require.NoError(t, err)
		assert.Equal(t, KindTransfusion, got)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := ParseKind("nope")

		require.ErrorIs(t, err, ErrUnknownKind)
	})
}

func TestNew(t *testing.T) {
	t.Run("sets default bases and Rome location", func(t *testing.T) {
		c := New(upstream.New(http.DefaultClient), newTokens("t", nil))

		assert.Equal(t, "https://api.regione.toscana.it/C04/fse-lab/v1/lab", c.bases[KindLab])
		assert.Equal(t, "https://apit.sanita.toscana.it/C06/fse-rap/v1", c.bases[KindPathology])
		assert.Len(t, c.bases, len(allKinds))
		assert.Equal(t, "Europe/Rome", c.loc.String())
	})
}

func listResponder(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lab/lista":
			write(t, w, `[{"actInternalIdAbstract":"A1","actInternalIdDettaglio":"D1","repCode":"R1","aziendaErogante":"ASL Test","dataPrestazione":"10/03/2024","dataAggiornamentoPrestazione":"12/03/2024","statoConsultazione":"0000","statoVisibilita":"0001"}]`)
		case "/ris/lista":
			write(t, w, `[{"actInternalIdAbstract":"A2","actInternalIdDettaglio":"D2","repCode":"R2","aziendaErogante":"Ospedale Test","dataPrestazione":"15/05/2024","statoConsultazione":"0001","statoVisibilita":"0002"}]`)
		default:
			write(t, w, `[]`)
		}
	}
}

func TestList(t *testing.T) {
	t.Run("merges kinds sorted by date desc with mapped fields", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, listResponder(t))

		got, err := c.List(context.Background())

		require.NoError(t, err)
		assert.Empty(t, got.Failures)
		require.Len(t, got.Reports, 2)
		assert.Equal(t, Report{
			Ref:      Ref{Kind: KindRadiology, AbstractID: "A2", DetailID: "D2", RepCode: "R2"},
			Date:     time.Date(2024, 5, 15, 0, 0, 0, 0, c.loc),
			Provider: "Ospedale Test",
			Hidden:   true,
		}, got.Reports[0])
		assert.Equal(t, Report{
			Ref:       Ref{Kind: KindLab, AbstractID: "A1", DetailID: "D1", RepCode: "R1"},
			Date:      time.Date(2024, 3, 10, 0, 0, 0, 0, c.loc),
			UpdatedAt: time.Date(2024, 3, 12, 0, 0, 0, 0, c.loc),
			Provider:  "ASL Test",
			Unread:    true,
		}, got.Reports[1])
	})

	t.Run("sends bearer header and empty body to every kind list", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, listResponder(t))

		_, err := c.List(context.Background())

		require.NoError(t, err)
		require.Len(t, rec.reqs, len(allKinds))
		for _, kind := range allKinds {
			req, ok := rec.find("/" + string(kind) + "/lista")
			require.True(t, ok)
			assert.Equal(t, "Bearer tok", req.Auth)
			assert.JSONEq(t, `{}`, req.Body)
		}
	})

	t.Run("per kind failure is kept in failures", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/ps/lista" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			listResponder(t)(w, r)
		})

		got, err := c.List(context.Background())

		require.NoError(t, err)
		require.Len(t, got.Failures, 1)
		assert.Equal(t, KindER, got.Failures[0].Kind)
		var statusErr *upstream.StatusError
		require.ErrorAs(t, got.Failures[0].Err, &statusErr)
		assert.Len(t, got.Reports, 2)
	})

	t.Run("all kinds failing returns error", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, err := c.List(context.Background())

		require.ErrorIs(t, err, ErrAllFailed)
	})

	t.Run("not logged in returns session error without upstream calls", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("", session.ErrNotLoggedIn), rec, listResponder(t))

		_, err := c.List(context.Background())

		require.ErrorIs(t, err, session.ErrNotLoggedIn)
		assert.Empty(t, rec.reqs)
	})
}

func TestDocuments(t *testing.T) {
	ref := Ref{AbstractID: "A1", DetailID: "D1", RepCode: "R1"}

	t.Run("lab array posts to dettaglio and skips entries without posizione", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `[{"posizione":"1","dataAccettazione":"01/02/2024","aziendaErogante":"ASL Test"},{"dataAccettazione":"01/02/2024"}]`)
		})
		ref := ref
		ref.Kind = KindLab

		got, err := c.Documents(context.Background(), ref)

		require.NoError(t, err)
		assert.Equal(t, []Document{{Position: "1", AcceptedAt: time.Date(2024, 2, 1, 0, 0, 0, 0, c.loc), Provider: "ASL Test"}}, got)
		req, ok := rec.find("/lab/dettaglio")
		require.True(t, ok)
		assert.Equal(t, "Bearer tok", req.Auth)
		assert.JSONEq(t, `{"actAbstractInternalId":"A1","actDettaglioInternalId":"D1","repCode":"R1"}`, req.Body)
	})

	t.Run("transfusion uses lab endpoints", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `[{"posizione":"2"}]`)
		})
		ref := ref
		ref.Kind = KindTransfusion

		got, err := c.Documents(context.Background(), ref)

		require.NoError(t, err)
		require.Len(t, got, 1)
		_, ok := rec.find("/lab/dettaglio")
		assert.True(t, ok)
	})

	t.Run("ldo array", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `[{"posizione":"3","dataInizioRicovero":"01/01/2024","descrizioneReparto":"Reparto"}]`)
		})
		ref := ref
		ref.Kind = KindDischarge

		got, err := c.Documents(context.Background(), ref)

		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "3", got[0].Position)
		_, ok := rec.find("/ldo/dettaglio")
		assert.True(t, ok)
	})

	t.Run("ris single object posts to abstract", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"posizione":"4","dataAccettazione":"05/06/2024","aziendaErogante":"Ospedale Test"}`)
		})
		ref := ref
		ref.Kind = KindRadiology

		got, err := c.Documents(context.Background(), ref)

		require.NoError(t, err)
		assert.Equal(t, []Document{{Position: "4", AcceptedAt: time.Date(2024, 6, 5, 0, 0, 0, 0, c.loc), Provider: "Ospedale Test"}}, got)
		_, ok := rec.find("/ris/abstract")
		assert.True(t, ok)
	})

	t.Run("ris array is accepted too", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `[{"posizione":"4"},{"posizione":"5"}]`)
		})
		ref := ref
		ref.Kind = KindRadiology

		got, err := c.Documents(context.Background(), ref)

		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("ps flattens lista with tipoVerbale labels", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `[{"azienda":"ASL Test","data":"07/08/2024","struttura":"PS","lista":[{"posizione":"p1","tipoVerbale":"Verbale PS"},{"posizione":"p2","tipoVerbale":"Referto"}]}]`)
		})
		ref := ref
		ref.Kind = KindER

		got, err := c.Documents(context.Background(), ref)

		require.NoError(t, err)
		accepted := time.Date(2024, 8, 7, 0, 0, 0, 0, c.loc)
		assert.Equal(t, []Document{
			{Position: "p1", Label: "Verbale PS", AcceptedAt: accepted, Provider: "ASL Test"},
			{Position: "p2", Label: "Referto", AcceptedAt: accepted, Provider: "ASL Test"},
		}, got)
		_, ok := rec.find("/ps/abstract")
		assert.True(t, ok)
	})

	t.Run("rsa without posizione returns one document with empty position", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"dataAccettazione":"09/09/2024","aziendaErogante":"ASL Test"}`)
		})
		ref := ref
		ref.Kind = KindSpecialist

		got, err := c.Documents(context.Background(), ref)

		require.NoError(t, err)
		assert.Equal(t, []Document{{AcceptedAt: time.Date(2024, 9, 9, 0, 0, 0, 0, c.loc), Provider: "ASL Test"}}, got)
		req, ok := rec.find("/rsa/dettaglio")
		require.True(t, ok)
		assert.JSONEq(t, `{"internalId":"D1","repCode":"R1"}`, req.Body)
	})

	t.Run("rap empty array still returns one empty document", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `[]`)
		})
		ref := ref
		ref.Kind = KindPathology

		got, err := c.Documents(context.Background(), ref)

		require.NoError(t, err)
		assert.Equal(t, []Document{{}}, got)
		_, ok := rec.find("/rap/dettaglio")
		assert.True(t, ok)
	})

	t.Run("unknown kind", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {})

		_, err := c.Documents(context.Background(), Ref{Kind: "zzz"})

		require.ErrorIs(t, err, ErrUnknownKind)
	})

	t.Run("not logged in", func(t *testing.T) {
		c := newTestClient(t, newTokens("", session.ErrNotLoggedIn), &recorder{}, func(w http.ResponseWriter, r *http.Request) {})

		_, err := c.Documents(context.Background(), Ref{Kind: KindLab})

		require.ErrorIs(t, err, session.ErrNotLoggedIn)
	})

	t.Run("upstream error is wrapped", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		})

		_, err := c.Documents(context.Background(), Ref{Kind: KindLab})

		var statusErr *upstream.StatusError
		require.ErrorAs(t, err, &statusErr)
	})

	t.Run("malformed documents body is an error", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `[{"posizione":1}]`)
		})

		_, err := c.Documents(context.Background(), Ref{Kind: KindLab})

		require.Error(t, err)
	})
}

func TestDownload(t *testing.T) {
	ref := Ref{AbstractID: "A1", DetailID: "D1", RepCode: "R1"}

	t.Run("lab posts posizione and decodes pdf", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"refertoCodificatoBase64":"`+pdfBase64+`"}`)
		})
		ref := ref
		ref.Kind = KindLab

		got, err := c.Download(context.Background(), ref, "1")

		require.NoError(t, err)
		assert.Equal(t, []byte("%PDF-1.4\n"), got)
		req, ok := rec.find("/lab/download")
		require.True(t, ok)
		assert.Equal(t, "Bearer tok", req.Auth)
		assert.JSONEq(t, `{"actAbstractInternalId":"A1","actDettaglioInternalId":"D1","repCode":"R1","posizione":"1"}`, req.Body)
	})

	t.Run("transfusion downloads through lab", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"refertoCodificatoBase64":"`+pdfBase64+`"}`)
		})
		ref := ref
		ref.Kind = KindTransfusion

		_, err := c.Download(context.Background(), ref, "1")

		require.NoError(t, err)
		_, ok := rec.find("/lab/download")
		assert.True(t, ok)
	})

	t.Run("ris uses its own download", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"refertoCodificatoBase64":"`+pdfBase64+`"}`)
		})
		ref := ref
		ref.Kind = KindRadiology

		_, err := c.Download(context.Background(), ref, "1")

		require.NoError(t, err)
		_, ok := rec.find("/ris/download")
		assert.True(t, ok)
	})

	t.Run("ldo uses its own download", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"refertoCodificatoBase64":"`+pdfBase64+`"}`)
		})
		ref := ref
		ref.Kind = KindDischarge

		_, err := c.Download(context.Background(), ref, "1")

		require.NoError(t, err)
		_, ok := rec.find("/ldo/download")
		assert.True(t, ok)
	})

	t.Run("ps uses downloadVerbale and verbale field", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"verbaleCodificatoBase64":"`+pdfBase64+`"}`)
		})
		ref := ref
		ref.Kind = KindER

		got, err := c.Download(context.Background(), ref, "p1")

		require.NoError(t, err)
		assert.Equal(t, []byte("%PDF-1.4\n"), got)
		req, ok := rec.find("/ps/downloadVerbale")
		require.True(t, ok)
		assert.JSONEq(t, `{"actAbstractInternalId":"A1","actDettaglioInternalId":"D1","repCode":"R1","posizione":"p1"}`, req.Body)
	})

	t.Run("rsa sends internalId and repCode only", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"refertoCodificatoBase64":"`+pdfBase64+`"}`)
		})
		ref := ref
		ref.Kind = KindSpecialist

		_, err := c.Download(context.Background(), ref, "")

		require.NoError(t, err)
		req, ok := rec.find("/rsa/download")
		require.True(t, ok)
		assert.JSONEq(t, `{"internalId":"D1","repCode":"R1"}`, req.Body)
	})

	t.Run("rap sends internalId and repCode only", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"refertoCodificatoBase64":"`+pdfBase64+`"}`)
		})
		ref := ref
		ref.Kind = KindPathology

		_, err := c.Download(context.Background(), ref, "")

		require.NoError(t, err)
		req, ok := rec.find("/rap/download")
		require.True(t, ok)
		assert.JSONEq(t, `{"internalId":"D1","repCode":"R1"}`, req.Body)
	})

	t.Run("empty repCode defaults", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"refertoCodificatoBase64":"`+pdfBase64+`"}`)
		})

		_, err := c.Download(context.Background(), Ref{Kind: KindRadiology, AbstractID: "A1", DetailID: "D1"}, "1")

		require.NoError(t, err)
		req, ok := rec.find("/ris/download")
		require.True(t, ok)
		assert.Contains(t, req.Body, `"repCode":"HDIS-RT:REP-900"`)
	})

	t.Run("non pdf content is rejected", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"refertoCodificatoBase64":"`+base64.StdEncoding.EncodeToString([]byte("<html>"))+`"}`)
		})

		_, err := c.Download(context.Background(), Ref{Kind: KindLab}, "1")

		require.ErrorIs(t, err, ErrNotPDF)
	})

	t.Run("empty payload is no document", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{}`)
		})

		_, err := c.Download(context.Background(), Ref{Kind: KindLab}, "1")

		require.ErrorIs(t, err, ErrNoDocument)
	})

	t.Run("invalid base64 is an error", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {
			write(t, w, `{"refertoCodificatoBase64":"!!!"}`)
		})

		_, err := c.Download(context.Background(), Ref{Kind: KindLab}, "1")

		require.Error(t, err)
	})

	t.Run("unknown kind", func(t *testing.T) {
		c := newTestClient(t, newTokens("tok", nil), &recorder{}, func(w http.ResponseWriter, r *http.Request) {})

		_, err := c.Download(context.Background(), Ref{Kind: "zzz"}, "1")

		require.ErrorIs(t, err, ErrUnknownKind)
	})

	t.Run("not logged in", func(t *testing.T) {
		c := newTestClient(t, newTokens("", session.ErrNotLoggedIn), &recorder{}, func(w http.ResponseWriter, r *http.Request) {})

		_, err := c.Download(context.Background(), Ref{Kind: KindLab}, "1")

		require.True(t, errors.Is(err, session.ErrNotLoggedIn))
	})
}

func TestFullFlowPaths(t *testing.T) {
	t.Run("list documents download for every kind hit only the allowed paths", func(t *testing.T) {
		rec := &recorder{}
		c := newTestClient(t, newTokens("tok", nil), rec, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/lista"):
				write(t, w, `[{"actInternalIdAbstract":"A","actInternalIdDettaglio":"D","repCode":"R","dataPrestazione":"01/01/2024"}]`)
			case r.URL.Path == "/ps/abstract":
				write(t, w, `[{"lista":[{"posizione":"p","tipoVerbale":"V"}]}]`)
			case strings.HasSuffix(r.URL.Path, "/downloadVerbale"):
				write(t, w, `{"verbaleCodificatoBase64":"`+pdfBase64+`"}`)
			case strings.HasSuffix(r.URL.Path, "/download"):
				write(t, w, `{"refertoCodificatoBase64":"`+pdfBase64+`"}`)
			default:
				write(t, w, `[{"posizione":"p"}]`)
			}
		})

		listing, err := c.List(context.Background())
		require.NoError(t, err)
		require.Len(t, listing.Reports, len(allKinds))
		for _, report := range listing.Reports {
			docs, err := c.Documents(context.Background(), report.Ref)
			require.NoError(t, err)
			require.NotEmpty(t, docs)
			_, err = c.Download(context.Background(), report.Ref, docs[0].Position)
			require.NoError(t, err)
		}

		got := make([]string, 0, len(rec.reqs))
		for _, req := range rec.reqs {
			got = append(got, req.Path)
		}
		assert.ElementsMatch(t, []string{
			"/lab/lista", "/labtrasf/lista", "/ris/lista", "/ldo/lista", "/ps/lista", "/rsa/lista", "/rap/lista",
			"/lab/dettaglio", "/lab/download",
			"/lab/dettaglio", "/lab/download",
			"/ris/abstract", "/ris/download",
			"/ldo/dettaglio", "/ldo/download",
			"/ps/abstract", "/ps/downloadVerbale",
			"/rsa/dettaglio", "/rsa/download",
			"/rap/dettaglio", "/rap/download",
		}, got)
	})
}
