package get

import (
	"errors"
	"net/http"

	storagepkg "url-shortener/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
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
			slog.String("request_id", chi.URLParam(r, "short_code")),
		)

		shortCode := chi.URLParam(r, "short_code")
		if shortCode == "" {
			log.Error("short_code missing")
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("short_code is required"))
			return
		}

		url, visits, source, cacheTTL, err := storage.GetURLWithVisits(shortCode)
		if errors.Is(err, storagepkg.ErrURLNotFound) {
			log.Info("short_code not found", slog.String("short_code", shortCode))
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, resp.Error("not found"))
			return
		}
		if err != nil {
			log.Error("failed to get url", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to get url"))
			return
		}

		render.JSON(w, r, Response{
			URL:             url,
			Visits:          visits,
			Source:          source,
			CacheTTLSeconds: cacheTTL,
		})
	}
}
