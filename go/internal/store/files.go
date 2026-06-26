package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type File struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Store) CreateFile(ctx context.Context, f *File, content io.Reader) error {
	f.CreatedAt = time.Now().UTC()

	path := filepath.Join(s.FilesDir(), f.ID)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	n, err := io.Copy(file, content)
	file.Close()
	if err != nil {
		os.Remove(path)
		return fmt.Errorf("write file: %w", err)
	}
	f.Size = n

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO files (id, filename, size, content_type, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, f.ID, f.Filename, f.Size, f.ContentType, f.CreatedAt)
	if err != nil {
		os.Remove(path)
		return fmt.Errorf("insert file record: %w", err)
	}
	return nil
}

func (s *Store) GetFile(ctx context.Context, id string) (File, error) {
	var f File
	err := s.db.QueryRowContext(ctx, `
		SELECT id, filename, size, content_type, created_at
		FROM files WHERE id = ?
	`, id).Scan(&f.ID, &f.Filename, &f.Size, &f.ContentType, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	if err != nil {
		return File{}, fmt.Errorf("query file: %w", err)
	}
	return f, nil
}

func (s *Store) OpenFile(ctx context.Context, id string) (*os.File, File, error) {
	f, err := s.GetFile(ctx, id)
	if err != nil {
		return nil, File{}, err
	}

	path := filepath.Join(s.FilesDir(), id)
	file, err := os.Open(path)
	if err != nil {
		return nil, File{}, fmt.Errorf("open file: %w", err)
	}
	return file, f, nil
}

func (s *Store) ListFiles(ctx context.Context) ([]File, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, filename, size, content_type, created_at
		FROM files
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query files: %w", err)
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.Filename, &f.Size, &f.ContentType, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}
