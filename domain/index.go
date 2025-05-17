package domain

import "errors"

var ErrIndexNotFound = errors.New("index not found")

type Index struct {
	Name   string
	Schema string // JSON文字列で保持
}

type IndexRepository interface {
	Create(index *Index) error
	FindByName(name string) (*Index, error)
	Exists(name string) (bool, error)
	List() ([]*Index, error)
	Delete(name string) error
}
