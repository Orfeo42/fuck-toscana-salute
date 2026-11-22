package facilities

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	ministryHost        = "www.dati.salute.gov.it"
	defaultMinistryBase = "https://" + ministryHost
	defaultCKANURL      = "https://dati.toscana.it/api/3/action/package_show?id=rt-strut-sanitarie"
	regioneFileSuffix   = "territorialifeature.csv"
	maxDownloadBytes    = 20 << 20
	userAgent           = "fuck-toscana-salute"
)

type pageData struct {
	Result struct {
		Data struct {
			AllNodeDataset struct {
				Nodes []struct {
					Relationships struct {
						Files []struct {
							URL string `json:"url"`
						} `json:"field_listafile"`
					} `json:"relationships"`
				} `json:"nodes"`
			} `json:"allNodeDataset"`
		} `json:"data"`
	} `json:"result"`
}

type ckanPackage struct {
	Result struct {
		Resources []struct {
			URL string `json:"url"`
		} `json:"resources"`
	} `json:"result"`
}

func (s *Service) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("facilities: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	client := s.client
	if req.URL.Hostname() == ministryHost {
		client = s.ministryClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facilities: download: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes+1))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("facilities: read download: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("facilities: close download: %w", closeErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrStatus, resp.StatusCode)
	}
	if len(data) > maxDownloadBytes {
		return nil, ErrTooLarge
	}
	return data, nil
}

func (s *Service) ministryFileURL(ctx context.Context, slug string) (string, error) {
	data, err := s.fetch(ctx, s.ministryBase+"/page-data/it/dataset/"+slug+"/page-data.json")
	if err != nil {
		return "", err
	}
	var page pageData
	if err := json.Unmarshal(data, &page); err != nil {
		return "", fmt.Errorf("facilities: decode page data: %w", err)
	}
	nodes := page.Result.Data.AllNodeDataset.Nodes
	if len(nodes) == 0 {
		return "", ErrNoCSV
	}
	for _, f := range nodes[0].Relationships.Files {
		if !strings.HasSuffix(strings.ToLower(f.URL), ".csv") {
			continue
		}
		return s.resolveURL(f.URL)
	}
	return "", ErrNoCSV
}

func (s *Service) resolveURL(raw string) (string, error) {
	base, err := url.Parse(s.ministryBase)
	if err != nil {
		return "", fmt.Errorf("facilities: parse base: %w", err)
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("facilities: parse file url: %w", err)
	}
	return base.ResolveReference(ref).String(), nil
}

func (s *Service) regioneFileURL(ctx context.Context) (string, error) {
	data, err := s.fetch(ctx, s.ckanURL)
	if err != nil {
		return "", err
	}
	var pkg ckanPackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("facilities: decode ckan package: %w", err)
	}
	for _, r := range pkg.Result.Resources {
		if strings.HasSuffix(strings.ToLower(r.URL), regioneFileSuffix) {
			return r.URL, nil
		}
	}
	return "", ErrNoResource
}

func (s *Service) loadMinistry(ctx context.Context) ([]Facility, error) {
	var all []Facility
	for _, sp := range ministrySpecs {
		fileURL, err := s.ministryFileURL(ctx, sp.slug)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sp.slug, err)
		}
		data, err := s.fetch(ctx, fileURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sp.slug, err)
		}
		parsed, err := parseMinistry(data, sp)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sp.slug, err)
		}
		all = append(all, parsed...)
	}
	return mergeMinistry(all), nil
}

func (s *Service) loadRegione(ctx context.Context) ([]Facility, error) {
	fileURL, err := s.regioneFileURL(ctx)
	if err != nil {
		return nil, err
	}
	data, err := s.fetch(ctx, fileURL)
	if err != nil {
		return nil, err
	}
	return parseRegione(data, time.Now())
}
