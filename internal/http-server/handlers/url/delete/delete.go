package delete

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
	resp.Response

	ShortCode string `json:"short_code"`
}

type URLDeleter interface {
	DeleteURL(alias string) error
}

func New(log *slog.Logger, storage URLDeleter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.delete.New"

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

		err := storage.DeleteURL(shortCode)
		if errors.Is(err, storagepkg.ErrURLNotFound) {
			log.Info("short_code not found", slog.String("short_code", shortCode))
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, resp.Error("short_code "+shortCode+" not found"))
			return
		}
		if err != nil {
			log.Error("failed to delete url", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to delete url"))
			return
		}

		render.JSON(w, r, Response{
			Response:  resp.OK(),
			ShortCode: shortCode,
		})
	}
}
