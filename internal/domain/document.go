package domain

import "errors"

var ErrDocumentNotFound = errors.New("document not found")

type Document struct {
	IndexName string
	Key       string
	Content   string // JSON文字列で保持
}

type DocumentRepository interface {
	Upsert(doc *Document) error
	Find(indexName, key string) (*Document, error)
	Delete(indexName, key string) error
	List(indexName string) ([]*Document, error)
	Count(indexName string) (int, error)
}
