package get

import (
	"encoding/json"
	"errors"
	"net/http"

	storagepkg "url-shortener/internal/storage"

	"golang.org/x/exp/slog"

	resp "url-shortener/internal/lib/api/response"
	"url-shortener/internal/lib/logger/sl"
)

type Response struct {
	URL              string  `json:"url"`
	Visits           int64   `json:"visits"`
	Source           string  `json:"source"`
	CacheTTLSeconds  float64 `json:"cache_ttl_seconds,omitempty"`
}

type URLGetter interface {
	GetURLWithVisits(shortCode string) (string, int64, string, float64, error)
}

func New(log *slog.Logger, storage URLGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.get.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", r.PathValue("short_code")),
		)

		shortCode := r.PathValue("short_code")
		if shortCode == "" {
			log.Error("short_code missing")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(resp.Error("short_code is required"))
			return
		}

		url, visits, source, cacheTTL, err := storage.GetURLWithVisits(shortCode)
		if errors.Is(err, storagepkg.ErrURLNotFound) {
			log.Info("short_code not found", slog.String("short_code", shortCode))
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(resp.Error("not found"))
			return
		}
		if err != nil {
			log.Error("failed to get url", sl.Err(err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(resp.Error("failed to get url"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			URL:             url,
			Visits:          visits,
			Source:          source,
			CacheTTLSeconds: cacheTTL,
		})
	}
}
