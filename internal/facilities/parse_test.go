package facilities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"
)

func latin1(t *testing.T, s string) []byte {
	t.Helper()
	out, err := charmap.ISO8859_1.NewEncoder().String(s)
	require.NoError(t, err)
	return []byte(out)
}

const apparecchiatureCSV = `codice_regione;regione;codice_azienda_sanitaria_asl;azienda_sanitaria_ASL;codice_struttura;denominazione_struttura;indirizzo_struttura;latitudine;longitudine;tipo_apparecchiatura
090;TOSCANA;090201;ASL TOSCANA SUD EST;  090001  ;  OSPEDALE SAN DONATO  ;  VIA FLEMING 1 - 52100 - Arezzo (AR)  ;43.4;11.8;TAC
090;TOSCANA;090201;ASL TOSCANA SUD EST;090001;OSPEDALE SAN DONATO;VIA FLEMING 1 - 52100 - Arezzo (AR);43.4;11.8;RMN
010;PIEMONTE;010201;ASL TO;010001;OSPEDALE TORINO;VIA ROMA 1 - 10100 - Torino (TO);45;7;TAC
090;TOSCANA;090202;ASL;090002;CLINICA SENZA TAIL;VIA ROMA;43;11;RMN
`

const ricoveroCSV = "anno;codice_struttura;denominazione_struttura;comune;sigla_provincia_struttura;codice_regione;Regione\n" +
	"2022;  090900  ;  PRESIDIO CITTÀ DI CASTELLO  ;  GROSSETO  ;  gr  ;090;TOSCANA\n" +
	"2022;010900;OSPEDALE ALTRO;TORINO;TO;010;PIEMONTE\n"

const caseDiCuraCSV = "Anno;Codice struttura;Denominazione struttura;Comune struttura;Sigla provincia struttura;Codice regione\n" +
	"2022;090500;CASA DI CURA VILLA SERENA;PISA;PI;090\n" +
	"2022;120500;CASA DI CURA LAZIO;ROMA;RM;120\n"

const aziendeCSV = "Anno;Codice Regione;Descrizione Regione;Codice Azienda;Codice struttura;Denominazione struttura;Indirizzo;Codice Comune;Comune;Sigla provincia\n" +
	"2021;090;TOSCANA;090901;090100;AOU VECCHIA;VIA VECCHIA 1;048017;FIRENZE;FI\n" +
	"2022;090;TOSCANA;090901;090101;AOU CAREGGI;  VIALE MORGAGNI 85  ;048017;  FIRENZE  ;FI\n" +
	"2023;010;PIEMONTE;010901;010101;AOU TORINO;VIA TORINO 1;001272;TORINO;TO\n"

const consultoriCSV = "Anno;Codice Regione;Descrizione Regione;Codice struttura;Denominazione struttura;Tipo struttura;Indirizzo;CAP;Codice Comune;Comune;Sigla provincia\n" +
	"2022;090;TOSCANA;090700;CONSULTORIO CENTRO;CONSULTORIO;VIA DEI MILLE 5;50100;048017;FIRENZE;FI\n" +
	"2022;050;VENETO;050700;CONSULTORIO VENETO;CONSULTORIO;VIA X 5;30100;027042;VENEZIA;VE\n"

const regioneCSV = `"Denominazione Struttura","Codice Struttura","Data Apertura","Data Chiusura","Indirizzo Struttura","Denominazione Comune","PROVINCIA","CAP","Denominazione Tipo Struttura Sanitaria"
"  AMBULATORIO ROSSI  ","R001","01/01/2000","31/12/9999","VIA DEI FIORI, 3","  PRATO  ","PO","59100","AMBULATORIO"
"AMBULATORIO CHIUSO","R002","01/01/2000","15/06/2012","VIA VECCHIA 1","PRATO","PO","59100","AMBULATORIO"
"AMBULATORIO ROSSI BIS","R001","01/01/2000","31/12/9999","VIA DEI FIORI, 3","PRATO","PO","59100","AMBULATORIO"
"STUDIO NERI","R003","01/01/2010","","VIA ROMA 7","LUCCA","LU","55100","STUDIO"
`

func TestParseMinistryApparecchiature(t *testing.T) {
	sp := ministrySpecs[0]

	t.Run("filters Toscana, trims, dedupes and splits the address tail", func(t *testing.T) {
		got, err := parseMinistry([]byte(apparecchiatureCSV), sp)
		require.NoError(t, err)
		assert.Equal(t, []Facility{
			{Code: "090001", Name: "OSPEDALE SAN DONATO", Comune: "Arezzo", Provincia: "AR", Address: "VIA FLEMING 1", Source: SourceMinistero},
			{Code: "090002", Name: "CLINICA SENZA TAIL", Address: "VIA ROMA", Source: SourceMinistero},
		}, got)
	})

	t.Run("reads the file as UTF-8", func(t *testing.T) {
		data := "codice_regione;codice_struttura;denominazione_struttura;indirizzo_struttura\n090;090003;OSPEDALE DELLA CITTÀ;VIA X 1\n"
		got, err := parseMinistry([]byte(data), sp)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "OSPEDALE DELLA CITTÀ", got[0].Name)
	})

	t.Run("missing required column fails", func(t *testing.T) {
		_, err := parseMinistry([]byte("codice_regione;altro\n090;x\n"), sp)
		require.ErrorIs(t, err, ErrMissingColumn)
	})

	t.Run("empty file fails", func(t *testing.T) {
		_, err := parseMinistry(nil, sp)
		require.ErrorIs(t, err, ErrEmptyFile)
	})
}

func TestParseMinistryRicovero(t *testing.T) {
	sp := ministrySpecs[1]

	t.Run("decodes latin1, trims and uppercases the province", func(t *testing.T) {
		got, err := parseMinistry(latin1(t, ricoveroCSV), sp)
		require.NoError(t, err)
		assert.Equal(t, []Facility{
			{Code: "090900", Name: "PRESIDIO CITTÀ DI CASTELLO", Comune: "GROSSETO", Provincia: "GR", Source: SourceMinistero},
		}, got)
	})
}

func TestParseMinistryCaseDiCura(t *testing.T) {
	sp := ministrySpecs[2]

	t.Run("finds columns by capitalised header and filters the region", func(t *testing.T) {
		got, err := parseMinistry(latin1(t, caseDiCuraCSV), sp)
		require.NoError(t, err)
		assert.Equal(t, []Facility{
			{Code: "090500", Name: "CASA DI CURA VILLA SERENA", Comune: "PISA", Provincia: "PI", Source: SourceMinistero},
		}, got)
	})
}

func TestParseMinistryAziende(t *testing.T) {
	sp := ministrySpecs[3]

	t.Run("keeps only the latest year of the region", func(t *testing.T) {
		got, err := parseMinistry(latin1(t, aziendeCSV), sp)
		require.NoError(t, err)
		assert.Equal(t, []Facility{
			{Code: "090101", Name: "AOU CAREGGI", Comune: "FIRENZE", Provincia: "FI", Address: "VIALE MORGAGNI 85", Source: SourceMinistero},
		}, got)
	})
}

func TestParseMinistryConsultori(t *testing.T) {
	sp := ministrySpecs[4]

	t.Run("filters the region and keeps the street address", func(t *testing.T) {
		got, err := parseMinistry(latin1(t, consultoriCSV), sp)
		require.NoError(t, err)
		assert.Equal(t, []Facility{
			{Code: "090700", Name: "CONSULTORIO CENTRO", Comune: "FIRENZE", Provincia: "FI", Address: "VIA DEI MILLE 5", Source: SourceMinistero},
		}, got)
	})
}

func TestParseRegione(t *testing.T) {
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)

	t.Run("decodes comma csv, drops closed and duplicate rows", func(t *testing.T) {
		got, err := parseRegione(latin1(t, regioneCSV), now)
		require.NoError(t, err)
		assert.Equal(t, []Facility{
			{Code: "R001", Name: "AMBULATORIO ROSSI", Comune: "PRATO", Provincia: "PO", Address: "VIA DEI FIORI, 3", Source: SourceRegione},
			{Code: "R003", Name: "STUDIO NERI", Comune: "LUCCA", Provincia: "LU", Address: "VIA ROMA 7", Source: SourceRegione},
		}, got)
	})

	t.Run("decodes latin1 accents", func(t *testing.T) {
		data := "Denominazione Struttura,Codice Struttura,Data Chiusura\nCLINICA CITTÀ,R010,31/12/9999\n"
		got, err := parseRegione(latin1(t, data), now)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "CLINICA CITTÀ", got[0].Name)
	})

	t.Run("missing column fails", func(t *testing.T) {
		_, err := parseRegione([]byte("Altro\nx\n"), now)
		require.ErrorIs(t, err, ErrMissingColumn)
	})
}

func TestMergeMinistry(t *testing.T) {
	t.Run("keeps the duplicate with an address", func(t *testing.T) {
		got := mergeMinistry([]Facility{
			{Code: "A1", Name: "Ospedale Città", Comune: "Siena"},
			{Code: "A2", Name: "OSPEDALE CITTA", Comune: "siena", Address: "VIA X 1"},
		})
		assert.Equal(t, []Facility{{Code: "A2", Name: "OSPEDALE CITTA", Comune: "siena", Address: "VIA X 1"}}, got)
	})

	t.Run("keeps the first when both have an address", func(t *testing.T) {
		got := mergeMinistry([]Facility{
			{Code: "A1", Name: "Ospedale", Comune: "Siena", Address: "VIA 1"},
			{Code: "A2", Name: "Ospedale", Comune: "Siena", Address: "VIA 2"},
		})
		assert.Equal(t, []Facility{{Code: "A1", Name: "Ospedale", Comune: "Siena", Address: "VIA 1"}}, got)
	})

	t.Run("same code with different names keeps one row", func(t *testing.T) {
		got := mergeMinistry([]Facility{
			{Code: "A1", Name: "Uno", Comune: "Siena"},
			{Code: "A1", Name: "Due", Comune: "Siena"},
		})
		assert.Equal(t, []Facility{{Code: "A1", Name: "Uno", Comune: "Siena"}}, got)
	})
}
