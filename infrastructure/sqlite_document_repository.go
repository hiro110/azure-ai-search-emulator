package infrastructure

import (
	"ai-search-emulator/domain"
	"database/sql"
)

type SQLiteDocumentRepository struct {
	db *sql.DB
}

func NewSQLiteDocumentRepository(db *sql.DB) *SQLiteDocumentRepository {
	return &SQLiteDocumentRepository{db: db}
}

func (r *SQLiteDocumentRepository) Upsert(doc *domain.Document) error {
	_, err := r.db.Exec("INSERT OR REPLACE INTO documents (index_name, key, content) VALUES (?, ?, ?)", doc.IndexName, doc.Key, doc.Content)
	return err
}

func (r *SQLiteDocumentRepository) Find(indexName, key string) (*domain.Document, error) {
	var doc domain.Document
	err := r.db.QueryRow("SELECT index_name, key, content FROM documents WHERE index_name = ? AND key = ?", indexName, key).Scan(&doc.IndexName, &doc.Key, &doc.Content)
	if err == sql.ErrNoRows {
		return nil, domain.ErrDocumentNotFound
	}
	return &doc, err
}

func (r *SQLiteDocumentRepository) Delete(indexName, key string) error {
	_, err := r.db.Exec("DELETE FROM documents WHERE index_name = ? AND key = ?", indexName, key)
	return err
}

func (r *SQLiteDocumentRepository) List(indexName string) ([]*domain.Document, error) {
	rows, err := r.db.Query("SELECT index_name, key, content FROM documents WHERE index_name = ?", indexName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*domain.Document
	for rows.Next() {
		var doc domain.Document
		if err := rows.Scan(&doc.IndexName, &doc.Key, &doc.Content); err != nil {
			return nil, err
		}
		result = append(result, &doc)
	}
	return result, nil
}

func (r *SQLiteDocumentRepository) Count(indexName string) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM documents WHERE index_name = ?", indexName).Scan(&count)
	return count, err
}
