package application

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"ai-search-emulator/domain"
)

// helper: returns an io.ReadCloser around a string body.
func body(s string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(s))
}

// helper: build a service backed by fresh in-memory mock repositories.
func newIndexServiceForTest() (*IndexService, *mockIndexRepository, *mockDocumentRepository) {
	idxRepo := newMockIndexRepository()
	docRepo := newMockDocumentRepository()
	return NewIndexService(idxRepo, docRepo), idxRepo, docRepo
}

const validSchemaJSON = `{
	"name": "my-index",
	"fields": [
		{"name": "id", "type": "Edm.String", "key": true},
		{"name": "title", "type": "Edm.String"}
	]
}`

func TestIndexService_CreateIndex_Success(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()

	if err := svc.CreateIndex(context.Background(), "my-index", body(validSchemaJSON)); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	got, err := idxRepo.FindByName("my-index")
	if err != nil {
		t.Fatalf("index should exist after creation: %v", err)
	}
	if got.Name != "my-index" {
		t.Errorf("name = %q, want %q", got.Name, "my-index")
	}
	if !strings.Contains(got.Schema, "\"fields\"") {
		t.Errorf("schema should contain fields, got %q", got.Schema)
	}
}

func TestIndexService_CreateIndex_AlreadyExists(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "my-index", Schema: validSchemaJSON})

	err := svc.CreateIndex(context.Background(), "my-index", body(validSchemaJSON))
	if err == nil || err.Error() != "already exists" {
		t.Fatalf("expected 'already exists' error, got %v", err)
	}
}

func TestIndexService_CreateIndex_InvalidJSON(t *testing.T) {
	t.Parallel()
	svc, _, _ := newIndexServiceForTest()

	err := svc.CreateIndex(context.Background(), "bad", body("{not-json"))
	if err == nil || !strings.Contains(err.Error(), "invalid schema json") {
		t.Fatalf("expected invalid schema error, got %v", err)
	}
}

func TestIndexService_CreateIndex_EmptyFields(t *testing.T) {
	t.Parallel()
	svc, _, _ := newIndexServiceForTest()

	err := svc.CreateIndex(context.Background(), "empty", body(`{"fields": []}`))
	if err == nil || !strings.Contains(err.Error(), "fields required") {
		t.Fatalf("expected fields-required error, got %v", err)
	}
}

func TestIndexService_CreateIndex_RepositoryExistsError(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	idxRepo.existsErr = errors.New("db down")

	err := svc.CreateIndex(context.Background(), "x", body(validSchemaJSON))
	if err == nil || err.Error() != "db down" {
		t.Fatalf("expected db down error, got %v", err)
	}
}

func TestIndexService_CreateIndex_BodyClosed(t *testing.T) {
	t.Parallel()
	svc, _, _ := newIndexServiceForTest()
	tc := &trackingCloser{ReadCloser: body(validSchemaJSON)}
	if err := svc.CreateIndex(context.Background(), "my-index", tc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tc.closed {
		t.Errorf("expected body to be closed by CreateIndex")
	}
}

type trackingCloser struct {
	io.ReadCloser
	closed bool
}

func (t *trackingCloser) Close() error {
	t.closed = true
	return t.ReadCloser.Close()
}

func TestIndexService_ListIndexes_Empty(t *testing.T) {
	t.Parallel()
	svc, _, _ := newIndexServiceForTest()

	got, err := svc.ListIndexes(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty list, got %d", len(got))
	}
}

func TestIndexService_ListIndexes_AllFields(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "a", Schema: validSchemaJSON})

	got, err := svc.ListIndexes(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 index, got %d", len(got))
	}
	if got[0]["name"] != "my-index" {
		t.Errorf("expected name=my-index, got %v", got[0]["name"])
	}
}

func TestIndexService_ListIndexes_SelectFilter(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "a", Schema: validSchemaJSON})

	got, err := svc.ListIndexes(context.Background(), "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 index, got %d", len(got))
	}
	if _, ok := got[0]["fields"]; ok {
		t.Errorf("fields should be filtered out when $select=name only")
	}
	if got[0]["name"] != "my-index" {
		t.Errorf("name not preserved, got %v", got[0]["name"])
	}
}

func TestIndexService_ListIndexes_SelectWithSpaces(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "a", Schema: validSchemaJSON})

	got, err := svc.ListIndexes(context.Background(), " name , fields ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0]["name"] != "my-index" {
		t.Errorf("name missing, got %v", got[0])
	}
	if _, ok := got[0]["fields"]; !ok {
		t.Errorf("fields should be present after trimming whitespace")
	}
}

func TestIndexService_ListIndexes_SkipsInvalidSchema(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	// Insert a record with broken JSON via the mock store directly.
	idxRepo.store["broken"] = &domain.Index{Name: "broken", Schema: "not-json"}
	_ = idxRepo.Create(&domain.Index{Name: "ok", Schema: validSchemaJSON})

	got, err := svc.ListIndexes(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 valid index (broken skipped), got %d", len(got))
	}
}

func TestIndexService_ListIndexes_RepoError(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	idxRepo.listErr = errors.New("boom")

	if _, err := svc.ListIndexes(context.Background(), ""); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestIndexService_GetIndex_Success(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "my-index", Schema: validSchemaJSON})

	got, err := svc.GetIndex(context.Background(), "my-index")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["name"] != "my-index" {
		t.Errorf("expected name=my-index, got %v", got["name"])
	}
}

func TestIndexService_GetIndex_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newIndexServiceForTest()
	_, err := svc.GetIndex(context.Background(), "missing")
	if err == nil || err.Error() != "index not found" {
		t.Fatalf("expected 'index not found', got %v", err)
	}
}

func TestIndexService_GetIndex_InvalidStoredSchema(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	idxRepo.store["broken"] = &domain.Index{Name: "broken", Schema: "{"}
	_, err := svc.GetIndex(context.Background(), "broken")
	if err == nil || !strings.Contains(err.Error(), "schema parse error") {
		t.Fatalf("expected schema parse error, got %v", err)
	}
}

func TestIndexService_GetIndex_RepoError(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	idxRepo.findErr = errors.New("boom")
	_, err := svc.GetIndex(context.Background(), "x")
	if err == nil || err.Error() == "index not found" {
		t.Fatalf("expected raw repo error to bubble up, got %v", err)
	}
}

func TestIndexService_UpdateIndex_Success(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "my-index", Schema: validSchemaJSON})

	updated := `{"name":"my-index","fields":[{"name":"id","key":true}]}`
	if err := svc.UpdateIndex(context.Background(), "my-index", body(updated)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := idxRepo.FindByName("my-index")
	if got.Schema != updated {
		t.Errorf("schema not updated: got %q", got.Schema)
	}
}

func TestIndexService_UpdateIndex_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newIndexServiceForTest()
	err := svc.UpdateIndex(context.Background(), "missing", body(validSchemaJSON))
	if err == nil || err.Error() != "index not found" {
		t.Fatalf("expected 'index not found', got %v", err)
	}
}

func TestIndexService_UpdateIndex_InvalidJSON(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "a", Schema: validSchemaJSON})

	err := svc.UpdateIndex(context.Background(), "a", body("garbage"))
	if err == nil || !strings.Contains(err.Error(), "invalid schema json") {
		t.Fatalf("expected invalid schema error, got %v", err)
	}
}

func TestIndexService_UpdateIndex_EmptyFields(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "a", Schema: validSchemaJSON})

	err := svc.UpdateIndex(context.Background(), "a", body(`{"fields":[]}`))
	if err == nil || !strings.Contains(err.Error(), "fields required") {
		t.Fatalf("expected fields-required error, got %v", err)
	}
}

func TestIndexService_DeleteIndex_Success(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "a", Schema: validSchemaJSON})

	if err := svc.DeleteIndex(context.Background(), "a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := idxRepo.FindByName("a"); err == nil {
		t.Errorf("index should be deleted")
	}
}

func TestIndexService_DeleteIndex_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newIndexServiceForTest()
	err := svc.DeleteIndex(context.Background(), "missing")
	if err == nil || err.Error() != "index not found" {
		t.Fatalf("expected 'index not found', got %v", err)
	}
}

func TestIndexService_DeleteIndex_RepoError(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newIndexServiceForTest()
	idxRepo.deleteErr = errors.New("db error")
	err := svc.DeleteIndex(context.Background(), "anything")
	if err == nil || err.Error() == "index not found" {
		t.Fatalf("expected db error to bubble up, got %v", err)
	}
}

func TestIndexService_GetIndexStats_Success(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "a", Schema: validSchemaJSON})
	_ = docRepo.Upsert(&domain.Document{IndexName: "a", Key: "1", Content: `{"id":"1"}`})
	_ = docRepo.Upsert(&domain.Document{IndexName: "a", Key: "2", Content: `{"id":"2"}`})

	stats, err := svc.GetIndexStats(context.Background(), "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats["documentCount"] != 2 {
		t.Errorf("documentCount = %v, want 2", stats["documentCount"])
	}
	if stats["storageSize"] != 0 {
		t.Errorf("storageSize = %v, want 0", stats["storageSize"])
	}
}

func TestIndexService_GetIndexStats_IndexNotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newIndexServiceForTest()
	_, err := svc.GetIndexStats(context.Background(), "missing")
	if err == nil || err.Error() != "index not found" {
		t.Fatalf("expected 'index not found', got %v", err)
	}
}

func TestIndexService_GetIndexStats_CountError(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newIndexServiceForTest()
	_ = idxRepo.Create(&domain.Index{Name: "a", Schema: validSchemaJSON})
	docRepo.countErr = errors.New("count failed")

	if _, err := svc.GetIndexStats(context.Background(), "a"); err == nil {
		t.Fatalf("expected count error, got nil")
	}
}
