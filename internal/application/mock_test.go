package application

import (
	"sync"

	"ai-search-emulator/internal/domain"
)

// mockIndexRepository is an in-memory implementation of domain.IndexRepository
// used for unit testing the application layer in isolation. It avoids any
// dependency on a real database driver.
//
// All methods are safe for concurrent use so that `go test -race` passes when
// tests exercise the repository from multiple goroutines.
type mockIndexRepository struct {
	mu    sync.RWMutex
	store map[string]*domain.Index

	// Optional error injectors so individual tests can simulate failure modes
	// without rewriting the whole mock.
	createErr     error
	updateErr     error
	findErr       error
	existsErr     error
	listErr       error
	deleteErr     error
}

func newMockIndexRepository() *mockIndexRepository {
	return &mockIndexRepository{store: map[string]*domain.Index{}}
}

func (m *mockIndexRepository) Create(index *domain.Index) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a fresh copy to avoid mutation of the caller's pointer.
	copy := &domain.Index{Name: index.Name, Schema: index.Schema}
	m.store[index.Name] = copy
	return nil
}

func (m *mockIndexRepository) Update(index *domain.Index) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store[index.Name]; !ok {
		return domain.ErrIndexNotFound
	}
	m.store[index.Name] = &domain.Index{Name: index.Name, Schema: index.Schema}
	return nil
}

func (m *mockIndexRepository) FindByName(name string) (*domain.Index, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx, ok := m.store[name]
	if !ok {
		return nil, domain.ErrIndexNotFound
	}
	// Return a copy to prevent the caller from mutating internal state.
	return &domain.Index{Name: idx.Name, Schema: idx.Schema}, nil
}

func (m *mockIndexRepository) Exists(name string) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.store[name]
	return ok, nil
}

func (m *mockIndexRepository) List() ([]*domain.Index, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*domain.Index, 0, len(m.store))
	for _, idx := range m.store {
		out = append(out, &domain.Index{Name: idx.Name, Schema: idx.Schema})
	}
	return out, nil
}

func (m *mockIndexRepository) Delete(name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store[name]; !ok {
		return domain.ErrIndexNotFound
	}
	delete(m.store, name)
	return nil
}

// mockDocumentRepository is an in-memory implementation of
// domain.DocumentRepository for application-layer unit tests.
type mockDocumentRepository struct {
	mu    sync.RWMutex
	store map[string]map[string]*domain.Document // indexName -> key -> doc

	upsertErr error
	findErr   error
	deleteErr error
	listErr   error
	countErr  error
	searchErr error
}

func newMockDocumentRepository() *mockDocumentRepository {
	return &mockDocumentRepository{store: map[string]map[string]*domain.Document{}}
}

func (m *mockDocumentRepository) Upsert(doc *domain.Document) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store[doc.IndexName]; !ok {
		m.store[doc.IndexName] = map[string]*domain.Document{}
	}
	m.store[doc.IndexName][doc.Key] = &domain.Document{
		IndexName: doc.IndexName,
		Key:       doc.Key,
		Content:   doc.Content,
	}
	return nil
}

func (m *mockDocumentRepository) Find(indexName, key string) (*domain.Document, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if docs, ok := m.store[indexName]; ok {
		if doc, ok := docs[key]; ok {
			return &domain.Document{
				IndexName: doc.IndexName,
				Key:       doc.Key,
				Content:   doc.Content,
			}, nil
		}
	}
	return nil, domain.ErrDocumentNotFound
}

func (m *mockDocumentRepository) Delete(indexName, key string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if docs, ok := m.store[indexName]; ok {
		delete(docs, key)
	}
	return nil
}

func (m *mockDocumentRepository) List(indexName string) ([]*domain.Document, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []*domain.Document{}
	if docs, ok := m.store[indexName]; ok {
		for _, doc := range docs {
			out = append(out, &domain.Document{
				IndexName: doc.IndexName,
				Key:       doc.Key,
				Content:   doc.Content,
			})
		}
	}
	return out, nil
}

func (m *mockDocumentRepository) Count(indexName string) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if docs, ok := m.store[indexName]; ok {
		return len(docs), nil
	}
	return 0, nil
}

// Search implements domain.DocumentRepository for unit tests.
// It applies in-memory text search and pagination; WhereSQL/OrderSQL are ignored
// (SQL-level filter behavior is tested via the SQLite repository integration tests).
func (m *mockDocumentRepository) Search(indexName string, opts domain.SearchOptions) ([]*domain.Document, int64, error) {
	if m.searchErr != nil {
		return nil, 0, m.searchErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []*domain.Document
	if docs, ok := m.store[indexName]; ok {
		for _, doc := range docs {
			if contains(doc.Content, opts.TextSearch) {
				all = append(all, &domain.Document{
					IndexName: doc.IndexName,
					Key:       doc.Key,
					Content:   doc.Content,
				})
			}
		}
	}

	total := int64(len(all))

	if opts.Skip > 0 {
		if opts.Skip >= len(all) {
			return []*domain.Document{}, total, nil
		}
		all = all[opts.Skip:]
	}
	top := opts.Top
	if top <= 0 {
		top = 50
	}
	if top < len(all) {
		all = all[:top]
	}
	return all, total, nil
}
