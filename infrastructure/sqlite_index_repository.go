package infrastructure

import (
	"ai-search-emulator/domain"
	"database/sql"
)

type SQLiteIndexRepository struct {
	db *sql.DB
}

func NewSQLiteIndexRepository(db *sql.DB) *SQLiteIndexRepository {
	return &SQLiteIndexRepository{db: db}
}

func (r *SQLiteIndexRepository) Create(index *domain.Index) error {
	_, err := r.db.Exec("INSERT INTO indexes (name, schema) VALUES (?, ?)", index.Name, index.Schema)
	return err
}

func (r *SQLiteIndexRepository) Exists(name string) (bool, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM indexes WHERE name = ?", name).Scan(&count)
	return count > 0, err
}

func (r *SQLiteIndexRepository) FindByName(name string) (*domain.Index, error) {
	var idx domain.Index
	err := r.db.QueryRow("SELECT name, schema FROM indexes WHERE name = ?", name).Scan(&idx.Name, &idx.Schema)
	if err == sql.ErrNoRows {
		return nil, domain.ErrIndexNotFound
	}
	return &idx, err
}

func (r *SQLiteIndexRepository) List() ([]*domain.Index, error) {
	rows, err := r.db.Query("SELECT name, schema FROM indexes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.Index
	for rows.Next() {
		var idx domain.Index
		if err := rows.Scan(&idx.Name, &idx.Schema); err != nil {
			return nil, err
		}
		result = append(result, &idx)
	}
	return result, nil
}

func (r *SQLiteIndexRepository) Delete(name string) error {
	_, err := r.db.Exec("DELETE FROM indexes WHERE name = ?", name)
	return err
}
