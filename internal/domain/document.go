package domain

import "errors"

var ErrDocumentNotFound = errors.New("document not found")
var ErrMissingKeyField = errors.New("missing key field")

type Document struct {
	IndexName string
	Key       string
	Content   string // JSON文字列で保持
}

// SearchOptions is passed to DocumentRepository.Search to specify query constraints.
// WhereSQL and WhereArgs are compiled from an OData $filter expression using
// json_extract() for SQLite. TextSearch is applied separately (LIKE on full content).
type SearchOptions struct {
	TextSearch       string        // raw search text; "" or "*" means no text filter
	TextSearchFields []string      // restrict text search to these fields; empty = full content
	WhereSQL         string        // compiled OData $filter SQL fragment (no WHERE keyword)
	WhereArgs        []interface{} // bind args for WhereSQL
	OrderSQL         string        // compiled OData $orderby SQL fragment (no ORDER BY keyword)
	Top              int           // must be > 0 (caller is responsible for applying the default)
	Skip             int
}

type DocumentRepository interface {
	Upsert(doc *Document) error
	Find(indexName, key string) (*Document, error)
	Delete(indexName, key string) error
	List(indexName string) ([]*Document, error)
	Count(indexName string) (int, error)
	// Search returns paginated documents matching opts and the total count before paging.
	Search(indexName string, opts SearchOptions) ([]*Document, int64, error)
}
