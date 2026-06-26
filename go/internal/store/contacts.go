package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Contact struct {
	ID        string    `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Company   string    `json:"company"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var ErrNotFound = errors.New("not found")

type PageToken struct {
	CreatedAt time.Time
	ID        string
}

func (s *Store) ListContacts(ctx context.Context, limit int, cursor *PageToken) ([]Contact, error) {
	var rows *sql.Rows
	var err error

	if cursor == nil {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, first_name, last_name, email, phone, company, created_at, updated_at
			FROM contacts
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, first_name, last_name, email, phone, company, created_at, updated_at
			FROM contacts
			WHERE (created_at, id) < (?, ?)
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		`, cursor.CreatedAt, cursor.ID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.FirstName, &c.LastName, &c.Email, &c.Phone, &c.Company, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

func (s *Store) GetContact(ctx context.Context, id string) (Contact, error) {
	var c Contact
	err := s.db.QueryRowContext(ctx, `
		SELECT id, first_name, last_name, email, phone, company, created_at, updated_at
		FROM contacts WHERE id = ?
	`, id).Scan(&c.ID, &c.FirstName, &c.LastName, &c.Email, &c.Phone, &c.Company, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Contact{}, ErrNotFound
	}
	if err != nil {
		return Contact{}, fmt.Errorf("query contact: %w", err)
	}
	return c, nil
}

func (s *Store) CreateContact(ctx context.Context, c *Contact) error {
	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO contacts (id, first_name, last_name, email, phone, company, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, c.ID, c.FirstName, c.LastName, c.Email, c.Phone, c.Company, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert contact: %w", err)
	}
	return nil
}

func (s *Store) UpdateContact(ctx context.Context, c *Contact) error {
	c.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx, `
		UPDATE contacts
		SET first_name = ?, last_name = ?, email = ?, phone = ?, company = ?, updated_at = ?
		WHERE id = ?
	`, c.FirstName, c.LastName, c.Email, c.Phone, c.Company, c.UpdatedAt, c.ID)
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteContact(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM contacts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CountContacts(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contacts`).Scan(&count)
	return count, err
}

func (s *Store) ImportContacts(ctx context.Context, contacts []Contact) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO contacts (id, first_name, last_name, email, phone, company, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	imported := 0
	for _, c := range contacts {
		c.CreatedAt = now
		c.UpdatedAt = now
		if _, err := stmt.ExecContext(ctx, c.ID, c.FirstName, c.LastName, c.Email, c.Phone, c.Company, c.CreatedAt, c.UpdatedAt); err != nil {
			return imported, fmt.Errorf("insert contact %s: %w", c.ID, err)
		}
		imported++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return imported, nil
}

func (s *Store) ExportContacts(ctx context.Context) ([]Contact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, first_name, last_name, email, phone, company, created_at, updated_at
		FROM contacts
		ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.FirstName, &c.LastName, &c.Email, &c.Phone, &c.Company, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}
