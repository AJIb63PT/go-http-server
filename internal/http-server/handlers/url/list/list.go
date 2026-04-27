package list

import (
	"encoding/json"
	"net/http"
	"strconv"

	"golang.org/x/exp/slog"

	resp "url-shortener/internal/lib/api/response"
	"url-shortener/internal/lib/logger/sl"
	"url-shortener/internal/storage"
)

type Response struct {
	resp.Response
	Links []storage.URL `json:"links"`
}

type URLLister interface {
	GetURLs(limit, offset int) ([]storage.URL, error)
}

func New(log *slog.Logger, storage URLLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.list.New"

		log := log.With(
			slog.String("op", op),
		)

		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		limit := 10
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		offset := 0
		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		urls, err := storage.GetURLs(limit, offset)
		if err != nil {
			log.Error("failed to get urls", sl.Err(err))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp.Error("failed to get urls"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Response: resp.OK(),
			Links:    urls,
		})
	}
}
