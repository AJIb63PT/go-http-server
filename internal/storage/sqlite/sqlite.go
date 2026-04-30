package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"url-shortener/internal/storage"
)

type cachedData struct {
	URL       string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Storage struct {
	db    *sql.DB
	cache sync.Map
}

func New(storagePath string) (*Storage, error) {
	const op = "storage.sqlite.New"

	db, err := sql.Open("sqlite3", storagePath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	storage := &Storage{db: db}

	if err := storage.Migrate(); err != nil {
		return nil, fmt.Errorf("%s: migrate: %w", op, err)
	}

	return storage, nil
}

func (s *Storage) Migrate() error {
	const op = "storage.sqlite.Migrate"

	stmt, err := s.db.Prepare(`
	CREATE TABLE IF NOT EXISTS url(
		id INTEGER PRIMARY KEY,
		short_code TEXT NOT NULL UNIQUE,
		original_url TEXT NOT NULL UNIQUE,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		visits INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_short_code ON url(short_code);
	`)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	_, err = stmt.Exec()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	stmt.Close()

	return nil
}

func (s *Storage) SaveURL(urlToSave string, shortCode string) (int64, error) {
	const op = "storage.sqlite.SaveURL"

	// Check if short_code already exists
	var existingID int64
	err := s.db.QueryRow("SELECT id FROM url WHERE short_code = ?", shortCode).Scan(&existingID)
	if err == nil {
		return 0, fmt.Errorf("%s: %w", op, storage.ErrShortCodeExists)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%s: check short_code: %w", op, err)
	}

	// Check if original_url already exists
	err = s.db.QueryRow("SELECT id FROM url WHERE original_url = ?", urlToSave).Scan(&existingID)
	if err == nil {
		return 0, fmt.Errorf("%s: %w", op, storage.ErrURLExists)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%s: check url: %w", op, err)
	}

	stmt, err := s.db.Prepare("INSERT INTO url(original_url, short_code) VALUES(?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(urlToSave, shortCode)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: failed to get last insert id: %w", op, err)
	}

	return id, nil
}

func (s *Storage) GetURL(shortCode string) (string, error) {
	const op = "storage.sqlite.GetURL"

	stmt, err := s.db.Prepare("SELECT original_url FROM url WHERE short_code = ?")
	if err != nil {
		return "", fmt.Errorf("%s: prepare statement: %w", op, err)
	}
	defer stmt.Close()

	var resURL string

	err = stmt.QueryRow(shortCode).Scan(&resURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", storage.ErrURLNotFound
		}

		return "", fmt.Errorf("%s: execute statement: %w", op, err)
	}

	return resURL, nil
}

func (s *Storage) GetURLStats(shortCode string) (*storage.URL, error) {
	const op = "storage.sqlite.GetURLStats"

	stmt, err := s.db.Prepare("SELECT id, short_code, original_url, created_at, visits FROM url WHERE short_code = ?")
	if err != nil {
		return nil, fmt.Errorf("%s: prepare statement: %w", op, err)
	}
	defer stmt.Close()

	var url storage.URL

	err = stmt.QueryRow(shortCode).Scan(&url.ID, &url.ShortCode, &url.OriginalURL, &url.CreatedAt, &url.Visits)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrURLNotFound
		}

		return nil, fmt.Errorf("%s: execute statement: %w", op, err)
	}

	return &url, nil
}

func (s *Storage) DeleteURL(shortCode string) error {
	const op = "storage.sqlite.DeleteURL"
	stmt, err := s.db.Prepare("DELETE FROM url WHERE short_code = ?")
	if err != nil {
		return fmt.Errorf("%s: prepare statement failed: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(shortCode)
	if err != nil {
		return fmt.Errorf("%s: execute failed: %w", op, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: failed to get rows affected: %w", op, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrURLNotFound)
	}

	s.cache.Delete(shortCode)

	return nil
}

func (s *Storage) GetURLs(limit, offset int) ([]storage.URL, error) {
	const op = "storage.sqlite.GetURLs"

	stmt, err := s.db.Prepare(`
		SELECT id, short_code, original_url, created_at, visits
		FROM url
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`)
	if err != nil {
		return nil, fmt.Errorf("%s: prepare statement: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%s: execute query: %w", op, err)
	}
	defer rows.Close()

	var urls []storage.URL
	for rows.Next() {
		var url storage.URL
		err := rows.Scan(&url.ID, &url.ShortCode, &url.OriginalURL, &url.CreatedAt, &url.Visits)
		if err != nil {
			return nil, fmt.Errorf("%s: scan row: %w", op, err)
		}
		urls = append(urls, url)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows error: %w", op, err)
	}

	return urls, nil
}

func (s *Storage) GetURLWithVisits(shortCode string) (string, int64, string, float64, error) {
	const op = "storage.sqlite.GetURLWithVisits"

	if val, ok := s.cache.Load(shortCode); ok {
		data := val.(cachedData)
		if time.Now().Before(data.ExpiresAt) {
			ttl := time.Until(data.ExpiresAt).Seconds()
			s.cache.Store(shortCode, data)
			res, err := s.db.Exec("UPDATE url SET visits = visits + 1 WHERE short_code = ?", shortCode)
			if err != nil {
				return "", 0, "", 0, fmt.Errorf("%s: update visits: %w", op, err)
			}
			rowsAffected, err := res.RowsAffected()
			if err != nil {
				return "", 0, "", 0, fmt.Errorf("%s: rows affected: %w", op, err)
			}
			if rowsAffected == 0 {
				return "", 0, "", 0, storage.ErrURLNotFound
			}

			var visits int64
			if err := s.db.QueryRow("SELECT visits FROM url WHERE short_code = ?", shortCode).Scan(&visits); err != nil {
				return "", 0, "", 0, fmt.Errorf("%s: select visits: %w", op, err)
			}

			return data.URL, visits, "cache", ttl, nil
		} else {
			s.cache.Delete(shortCode)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", 0, "", 0, fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.Exec("UPDATE url SET visits = visits + 1 WHERE short_code = ?", shortCode)
	if err != nil {
		return "", 0, "", 0, fmt.Errorf("%s: update visits: %w", op, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return "", 0, "", 0, fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if rowsAffected == 0 {
		return "", 0, "", 0, storage.ErrURLNotFound
	}

	var url string
	var visits int64
	var createdAt time.Time
	row := tx.QueryRow("SELECT original_url, visits, created_at FROM url WHERE short_code = ?", shortCode)
	if err := row.Scan(&url, &visits, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, "", 0, storage.ErrURLNotFound
		}
		return "", 0, "", 0, fmt.Errorf("%s: select url: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return "", 0, "", 0, fmt.Errorf("%s: commit: %w", op, err)
	}

	s.cache.Store(shortCode, cachedData{
		URL:       url,
		CreatedAt: createdAt,
		ExpiresAt: time.Now().Add(10 * time.Second),
	})

	return url, visits, "db", 0, nil
}
