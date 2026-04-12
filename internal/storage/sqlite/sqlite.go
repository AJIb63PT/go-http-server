package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"

	"url-shortener/internal/storage"
)

type Storage struct {
	db *sql.DB
}

func New(storagePath string) (*Storage, error) {
	const op = "storage.sqlite.New"

	db, err := sql.Open("sqlite3", storagePath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	stmt, err := db.Prepare(`
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
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	_, err = stmt.Exec()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	stmt.Close()

	return &Storage{db: db}, nil
}

func (s *Storage) SaveURL(urlToSave string, alias string) (int64, error) {
	const op = "storage.sqlite.SaveURL"

	stmt, err := s.db.Prepare("INSERT INTO url(original_url, short_code) VALUES(?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(urlToSave, alias)
	if err != nil {
		if sqliteErr, ok := err.(sqlite3.Error); ok && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrURLExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: failed to get last insert id: %w", op, err)
	}

	return id, nil
}

func (s *Storage) GetURL(alias string) (string, error) {
	const op = "storage.sqlite.GetURL"

	stmt, err := s.db.Prepare("SELECT original_url FROM url WHERE short_code = ?")
	if err != nil {
		return "", fmt.Errorf("%s: prepare statement: %w", op, err)
	}
	defer stmt.Close()

	var resURL string

	err = stmt.QueryRow(alias).Scan(&resURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", storage.ErrURLNotFound
		}

		return "", fmt.Errorf("%s: execute statement: %w", op, err)
	}

	return resURL, nil
}

func (s *Storage) DeleteURL(alias string) error {
	const op = "storage.sqlite.DeleteURL"
	stmt, err := s.db.Prepare("DELETE FROM url WHERE short_code = ?")
	if err != nil {
		return fmt.Errorf("%s: prepare statement failed: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(alias)
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

func (s *Storage) GetURLWithVisits(shortCode string) (string, int64, error) {
	const op = "storage.sqlite.GetURLWithVisits"

	tx, err := s.db.Begin()
	if err != nil {
		return "", 0, fmt.Errorf("%s: begin transaction: %w", op, err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	res, err := tx.Exec("UPDATE url SET visits = visits + 1 WHERE short_code = ?", shortCode)
	if err != nil {
		return "", 0, fmt.Errorf("%s: update visits: %w", op, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return "", 0, fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if rowsAffected == 0 {
		return "", 0, storage.ErrURLNotFound
	}

	var url string
	var visits int64
	row := tx.QueryRow("SELECT original_url, visits FROM url WHERE short_code = ?", shortCode)
	if err := row.Scan(&url, &visits); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, storage.ErrURLNotFound
		}
		return "", 0, fmt.Errorf("%s: select url: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return "", 0, fmt.Errorf("%s: commit: %w", op, err)
	}

	return url, visits, nil
}
