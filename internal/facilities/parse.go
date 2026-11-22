package facilities

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

const (
	byteOrderMark     = "\uFEFF"
	toscanaRegionCode = "090"
	regioneDateLayout = "02/01/2006"
)

var addressTail = regexp.MustCompile(`^(.*?)\s*-\s*\d{5}\s*-\s*(.+?)\s*\(([A-Za-z]{2})\)$`)

type columns struct {
	code, name, comune, provincia, address, region, year string
}

type spec struct {
	slug        string
	utf8        bool
	addressTail bool
	cols        columns
}

var ministrySpecs = []spec{
	{
		slug:        "apparecchiature-sanitarie",
		utf8:        true,
		addressTail: true,
		cols:        columns{code: "codice struttura", name: "denominazione struttura", address: "indirizzo struttura", region: "codice regione"},
	},
	{
		slug: "strutture-di-ricovero-pubbliche-e-equiparate-presenti-nel-territorio-della-asl",
		cols: columns{code: "codice struttura", name: "denominazione struttura", comune: "comune", provincia: "sigla provincia struttura", region: "codice regione"},
	},
	{
		slug: "case-di-cura-accreditate-presenti-nel-territorio-della-asl",
		cols: columns{code: "codice struttura", name: "denominazione struttura", comune: "comune struttura", provincia: "sigla provincia struttura", region: "codice regione"},
	},
	{
		slug: "aziende-ospedaliere-aziende-ospedaliere-universitarie-e-irccs-pubblici-anche-costituiti",
		cols: columns{code: "codice struttura", name: "denominazione struttura", comune: "comune", provincia: "sigla provincia", address: "indirizzo", region: "codice regione", year: "anno"},
	},
	{
		slug: "consultori-familiari-anno-2022",
		cols: columns{code: "codice struttura", name: "denominazione struttura", comune: "comune", provincia: "sigla provincia", address: "indirizzo", region: "codice regione"},
	},
}

var regioneColumns = columns{code: "codice struttura", name: "denominazione struttura", comune: "denominazione comune", provincia: "provincia", address: "indirizzo struttura"}

type table struct {
	header map[string]int
	rows   [][]string
}

func normalizeHeader(h string) string {
	h = strings.TrimPrefix(h, byteOrderMark)
	h = strings.ReplaceAll(h, "_", " ")
	return strings.Join(strings.Fields(strings.ToLower(h)), " ")
}

func readTable(data []byte, comma rune, latin1 bool) (table, error) {
	var r io.Reader = bytes.NewReader(data)
	if latin1 {
		r = charmap.ISO8859_1.NewDecoder().Reader(r)
	}
	cr := csv.NewReader(r)
	cr.Comma = comma
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return table{}, fmt.Errorf("facilities: read csv: %w", err)
	}
	if len(records) == 0 {
		return table{}, ErrEmptyFile
	}
	header := make(map[string]int, len(records[0]))
	for i, h := range records[0] {
		header[normalizeHeader(h)] = i
	}
	return table{header: header, rows: records[1:]}, nil
}

func (t table) has(col string) bool {
	_, ok := t.header[col]
	return ok
}

func (t table) get(row []string, col string) string {
	i, ok := t.header[col]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func (t table) require(cols ...string) error {
	for _, c := range cols {
		if !t.has(c) {
			return fmt.Errorf("%w: %s", ErrMissingColumn, c)
		}
	}
	return nil
}

func (t table) facility(row []string, c columns, source string) Facility {
	f := Facility{
		Code:      t.get(row, c.code),
		Name:      t.get(row, c.name),
		Comune:    t.get(row, c.comune),
		Provincia: t.get(row, c.provincia),
		Address:   t.get(row, c.address),
		Source:    source,
	}
	f.Provincia = strings.ToUpper(f.Provincia)
	return f
}

func splitAddress(f Facility) Facility {
	m := addressTail.FindStringSubmatch(f.Address)
	if m == nil {
		return f
	}
	f.Address = m[1]
	f.Comune = m[2]
	f.Provincia = strings.ToUpper(m[3])
	return f
}

func parseMinistry(data []byte, sp spec) ([]Facility, error) {
	t, err := readTable(data, ';', !sp.utf8)
	if err != nil {
		return nil, err
	}
	if err := t.require(sp.cols.code, sp.cols.name, sp.cols.region); err != nil {
		return nil, err
	}
	rows := t.rows
	if sp.cols.year != "" {
		rows = latestYear(t, sp.cols)
	}
	seen := make(map[string]bool)
	var out []Facility
	for _, row := range rows {
		if t.get(row, sp.cols.region) != toscanaRegionCode {
			continue
		}
		f := t.facility(row, sp.cols, SourceMinistero)
		if f.Code == "" || f.Name == "" || seen[f.Code] {
			continue
		}
		seen[f.Code] = true
		if sp.addressTail {
			f = splitAddress(f)
		}
		out = append(out, f)
	}
	return out, nil
}

func latestYear(t table, c columns) [][]string {
	latest := 0
	for _, row := range t.rows {
		if t.get(row, c.region) != toscanaRegionCode {
			continue
		}
		latest = max(latest, rowYear(t, row, c))
	}
	var out [][]string
	for _, row := range t.rows {
		if rowYear(t, row, c) == latest {
			out = append(out, row)
		}
	}
	return out
}

func rowYear(t table, row []string, c columns) int {
	year, err := strconv.Atoi(t.get(row, c.year))
	if err != nil {
		return 0
	}
	return year
}

func parseRegione(data []byte, now time.Time) ([]Facility, error) {
	t, err := readTable(data, ',', true)
	if err != nil {
		return nil, err
	}
	c := regioneColumns
	if err := t.require(c.code, c.name); err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []Facility
	for _, row := range t.rows {
		f := t.facility(row, c, SourceRegione)
		if f.Code == "" || f.Name == "" || seen[f.Code] {
			continue
		}
		closed, err := time.Parse(regioneDateLayout, t.get(row, "data chiusura"))
		if err == nil && closed.Before(now) {
			continue
		}
		seen[f.Code] = true
		out = append(out, f)
	}
	return out, nil
}
