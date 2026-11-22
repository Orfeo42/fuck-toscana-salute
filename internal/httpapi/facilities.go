package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type searchFacilitiesInput struct {
	Body SearchFacilitiesRequest
}

type SearchFacilitiesRequest struct {
	Query string `json:"query" required:"true" minLength:"2" maxLength:"100"`
}

type searchFacilitiesOutput struct {
	Body FacilitySearchBody
}

type FacilitySearchBody struct {
	Facilities []FacilityBody `json:"facilities"`
}

type FacilityBody struct {
	Name      string `json:"name" required:"true"`
	Comune    string `json:"comune" required:"true"`
	Provincia string `json:"provincia" required:"true"`
	Address   string `json:"address" required:"true"`
	Source    string `json:"source" required:"true" enum:"ministero,regione"`
}

func registerFacilities(api huma.API, fac Facilities) {
	huma.Register(api, huma.Operation{
		OperationID:   "search-facilities",
		Method:        http.MethodPost,
		Path:          "/api/facilities/search",
		DefaultStatus: http.StatusOK,
		Middlewares:   sameOrigin(api),
	}, func(ctx context.Context, in *searchFacilitiesInput) (*searchFacilitiesOutput, error) {
		found, err := fac.Search(ctx, in.Body.Query)
		if err != nil {
			return nil, toHTTPError(ctx, "search-facilities", err)
		}
		body := FacilitySearchBody{Facilities: make([]FacilityBody, 0, len(found))}
		for _, f := range found {
			body.Facilities = append(body.Facilities, FacilityBody{Name: f.Name, Comune: f.Comune, Provincia: f.Provincia, Address: f.Address, Source: f.Source})
		}
		slog.InfoContext(ctx, "facilities searched", slog.Int("results", len(body.Facilities)))
		return &searchFacilitiesOutput{Body: body}, nil
	})
}
