package infrastructure

import (
	"ai-search-emulator/internal/domain"
	"database/sql"
	"fmt"
	"strings"
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

// Search executes a filtered, ordered, paginated query against the documents table.
// Total count is computed before paging so the caller can include it in $count responses.
func (r *SQLiteDocumentRepository) Search(indexName string, opts domain.SearchOptions) ([]*domain.Document, int64, error) {
	where, args := r.buildWhere(indexName, opts)

	var total int64
	countSQL := "SELECT COUNT(*) FROM documents WHERE " + where
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search count: %w", err)
	}

	mainSQL := "SELECT index_name, key, content FROM documents WHERE " + where
	if opts.OrderSQL != "" {
		mainSQL += " ORDER BY " + opts.OrderSQL
	}
	top := opts.Top
	if top <= 0 {
		top = 50
	}
	mainSQL += fmt.Sprintf(" LIMIT %d OFFSET %d", top, opts.Skip)

	rows, err := r.db.Query(mainSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	var result []*domain.Document
	for rows.Next() {
		var doc domain.Document
		if err := rows.Scan(&doc.IndexName, &doc.Key, &doc.Content); err != nil {
			return nil, 0, err
		}
		result = append(result, &doc)
	}
	return result, total, rows.Err()
}

// buildWhere constructs the WHERE clause and bind args for Search.
func (r *SQLiteDocumentRepository) buildWhere(indexName string, opts domain.SearchOptions) (string, []interface{}) {
	parts := []string{"index_name = ?"}
	args := []interface{}{indexName}

	if opts.TextSearch != "" && opts.TextSearch != "*" {
		lower := strings.ToLower(opts.TextSearch)
		if len(opts.TextSearchFields) > 0 {
			fieldParts := make([]string, len(opts.TextSearchFields))
			for i, f := range opts.TextSearchFields {
				fieldParts[i] = fmt.Sprintf("LOWER(json_extract(content, '$.%s')) LIKE ?", f)
				args = append(args, "%"+lower+"%")
			}
			parts = append(parts, "("+strings.Join(fieldParts, " OR ")+")")
		} else {
			parts = append(parts, "LOWER(content) LIKE ?")
			args = append(args, "%"+lower+"%")
		}
	}

	if opts.WhereSQL != "" {
		parts = append(parts, "("+opts.WhereSQL+")")
		args = append(args, opts.WhereArgs...)
	}

	return strings.Join(parts, " AND "), args
}
