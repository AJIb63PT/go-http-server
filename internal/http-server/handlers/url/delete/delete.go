package delete

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
	resp.Response

	ShortCode string `json:"short_code"`
}

type URLDeleter interface {
	DeleteURL(shortCode string) error
}

func New(log *slog.Logger, storage URLDeleter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.delete.New"

		log := log.With(
			slog.String("op", op),
			slog.String("short_code", r.PathValue("short_code")),
		)

		shortCode := r.PathValue("short_code")
		if shortCode == "" {
			log.Error("short_code missing")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(resp.Error("short_code is required"))
			return
		}

		err := storage.DeleteURL(shortCode)
		if errors.Is(err, storagepkg.ErrURLNotFound) {
			log.Info("short_code not found", slog.String("short_code", shortCode))
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(resp.Error("short_code " + shortCode + " not found"))
			return
		}
		if err != nil {
			log.Error("failed to delete url", sl.Err(err))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp.Error("failed to delete url"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Response:  resp.OK(),
			ShortCode: shortCode,
		})
	}
}
