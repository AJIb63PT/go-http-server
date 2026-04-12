package storage

import (
	"errors"
	"time"
)

type URL struct {
	ID         int64     `json:"id"`
	ShortCode  string    `json:"short_code"`
	OriginalURL string   `json:"original_url"`
	CreatedAt  time.Time `json:"created_at"`
	Visits     int64     `json:"visits"`
}

var (
	ErrURLNotFound = errors.New("url not found")
	ErrURLExists   = errors.New("url exists")
)
