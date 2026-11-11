package httpapi

import (
	"context"
	"log/slog"
	"time"
)

func fetchedAt(ctx context.Context, fresh Freshness, source string) map[string]time.Time {
	times, err := fresh.FetchedAt(ctx, source)
	if err != nil {
		slog.WarnContext(ctx, "freshness unavailable", slog.String("source", source), slog.String("error", err.Error()))
		return map[string]time.Time{}
	}
	if times == nil {
		return map[string]time.Time{}
	}
	return times
}

func listUpdated(ctx context.Context, fresh Freshness, sources ...string) *time.Time {
	var oldest *time.Time
	for _, source := range sources {
		at, ok := fetchedAt(ctx, fresh, source)[""]
		if !ok {
			continue
		}
		if oldest == nil || at.Before(*oldest) {
			oldest = &at
		}
	}
	return oldest
}
