package stats

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
	ShortCode string `json:"short_code"`
	URL       string `json:"url"`
	Visits    int64  `json:"visits"`
	CreatedAt string `json:"created_at"`
}

type URLStatsGetter interface {
	GetURLStats(shortCode string) (*storagepkg.URL, error)
}

func New(log *slog.Logger, storage URLStatsGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.stats.New"

		log := log.With(
			slog.String("op", op),
			slog.String("short_code", chi.URLParam(r, "short_code")),
		)

		shortCode := chi.URLParam(r, "short_code")
		if shortCode == "" {
			log.Error("short_code missing")
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("short_code is required"))
			return
		}

		url, err := storage.GetURLStats(shortCode)
		if errors.Is(err, storagepkg.ErrURLNotFound) {
			log.Info("short_code not found", slog.String("short_code", shortCode))
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, resp.Error("not found"))
			return
		}
		if err != nil {
			log.Error("failed to get url stats", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to get url stats"))
			return
		}

		render.JSON(w, r, Response{
			ShortCode: url.ShortCode,
			URL:       url.OriginalURL,
			Visits:    url.Visits,
			CreatedAt: url.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), // ISO 8601 format
		})
	}
}
