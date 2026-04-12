package save

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"golang.org/x/exp/slog"

	resp "url-shortener/internal/lib/api/response"
	"url-shortener/internal/lib/logger/sl"
	"url-shortener/internal/lib/random"
	"url-shortener/internal/storage"
)

type Request struct {
	URL       string `json:"url" validate:"required,url"`
	ShortCode string `json:"short_code,omitempty"`
}

type Response struct {
	resp.Response
	ShortCode string `json:"short_code,omitempty"`
}

// TODO: move to config if needed
const shortCodeLength = 6

type URLSaver interface {
	SaveURL(urlToSave string, shortCode string) (int64, error)
}

func New(log *slog.Logger, urlSaver URLSaver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.save.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req Request

		err := render.DecodeJSON(r.Body, &req)
		if errors.Is(err, io.EOF) {
			// Такую ошибку встретим, если получили запрос с пустым телом.
			// Обработаем её отдельно
			log.Error("request body is empty")

			render.JSON(w, r, resp.Error("empty request"))

			return
		}
		if err != nil {
			log.Error("failed to decode request body", sl.Err(err))

			render.JSON(w, r, resp.Error("failed to decode request"))

			return
		}

		log.Info("request body decoded", slog.Any("request", req))

		if err := validator.New().Struct(req); err != nil {
			validateErr := err.(validator.ValidationErrors)

			log.Error("invalid request", sl.Err(err))

			render.JSON(w, r, resp.ValidationError(validateErr))

			return
		}

		shortCode := req.ShortCode
		var id int64

		if shortCode == "" {
			for i := 0; i < 5; i++ {
				shortCode = random.NewRandomString(shortCodeLength)
				id, err = urlSaver.SaveURL(req.URL, shortCode)
				if err == nil {
					break
				}
				if !errors.Is(err, storage.ErrURLExists) {
					log.Error("failed to add url", sl.Err(err))
					render.JSON(w, r, resp.Error("failed to add url"))
					return
				}
			}
			if err != nil {
				log.Info("url already exists", slog.String("url", req.URL))
				render.JSON(w, r, resp.Error("url already exists"))
				return
			}
		} else {
			id, err = urlSaver.SaveURL(req.URL, shortCode)
			if errors.Is(err, storage.ErrURLExists) {
				log.Info("short_code already exists", slog.String("short_code", shortCode))
				render.JSON(w, r, resp.Error("short_code already exists"))
				return
			}
			if err != nil {
				log.Error("failed to add url", sl.Err(err))
				render.JSON(w, r, resp.Error("failed to add url"))
				return
			}
		}

		log.Info("url added", slog.Int64("id", id))

		responseOK(w, r, shortCode)
	}
}

func responseOK(w http.ResponseWriter, r *http.Request, shortCode string) {
	render.JSON(w, r, Response{
		Response:  resp.OK(),
		ShortCode: shortCode,
	})
}
