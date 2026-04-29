package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"ai-search-emulator/internal/domain"
)

func newDocumentServiceForTest() (*DocumentService, *mockIndexRepository, *mockDocumentRepository) {
	idxRepo := newMockIndexRepository()
	docRepo := newMockDocumentRepository()
	return NewDocumentService(docRepo, idxRepo), idxRepo, docRepo
}

// seedIndex inserts an index into the mock repository with the schema used for
// most tests (key field is "id").
func seedIndex(t *testing.T, idxRepo *mockIndexRepository, name string) {
	t.Helper()
	if err := idxRepo.Create(&domain.Index{Name: name, Schema: validSchemaJSON}); err != nil {
		t.Fatalf("failed to seed index: %v", err)
	}
}

// --- AddOrUpdateSingleDoc ---

func TestDocumentService_AddOrUpdateSingleDoc_Success(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")

	doc := map[string]interface{}{"id": "doc-1", "title": "hello"}
	if err := svc.AddOrUpdateSingleDoc(context.Background(), "idx", doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := docRepo.Find("idx", "doc-1")
	if err != nil {
		t.Fatalf("expected document persisted, got %v", err)
	}
	var content map[string]interface{}
	_ = json.Unmarshal([]byte(got.Content), &content)
	if content["title"] != "hello" {
		t.Errorf("title = %v, want hello", content["title"])
	}
}

func TestDocumentService_AddOrUpdateSingleDoc_IndexNotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newDocumentServiceForTest()
	err := svc.AddOrUpdateSingleDoc(context.Background(), "missing", map[string]interface{}{"id": "1"})
	if err == nil || err.Error() != "index not found" {
		t.Fatalf("expected 'index not found', got %v", err)
	}
}

func TestDocumentService_AddOrUpdateSingleDoc_MissingKeyField(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	err := svc.AddOrUpdateSingleDoc(context.Background(), "idx", map[string]interface{}{"title": "no key"})
	if err == nil || err.Error() != "missing key field" {
		t.Fatalf("expected 'missing key field', got %v", err)
	}
}

func TestDocumentService_AddOrUpdateSingleDoc_NonStringKey(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	err := svc.AddOrUpdateSingleDoc(context.Background(), "idx", map[string]interface{}{"id": 42, "title": "n"})
	if err == nil || err.Error() != "key field must be string" {
		t.Fatalf("expected 'key field must be string', got %v", err)
	}
}

func TestDocumentService_AddOrUpdateSingleDoc_NoKeyInSchema(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	// Schema with no key=true field.
	_ = idxRepo.Create(&domain.Index{
		Name:   "no-key",
		Schema: `{"fields":[{"name":"id","key":false}]}`,
	})
	err := svc.AddOrUpdateSingleDoc(context.Background(), "no-key", map[string]interface{}{"id": "1"})
	if err == nil || err.Error() != "missing key field" {
		t.Fatalf("expected 'missing key field', got %v", err)
	}
}

func TestDocumentService_AddOrUpdateSingleDoc_BadSchema(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	idxRepo.store["bad"] = &domain.Index{Name: "bad", Schema: "not-json"}
	err := svc.AddOrUpdateSingleDoc(context.Background(), "bad", map[string]interface{}{"id": "1"})
	if err == nil || !strings.Contains(err.Error(), "schema parse error") {
		t.Fatalf("expected schema parse error, got %v", err)
	}
}

func TestDocumentService_AddOrUpdateSingleDoc_ExistsRepoError(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	idxRepo.existsErr = errors.New("db down")
	err := svc.AddOrUpdateSingleDoc(context.Background(), "x", map[string]interface{}{"id": "1"})
	if err == nil || err.Error() != "db down" {
		t.Fatalf("expected db down, got %v", err)
	}
}

// --- BatchOperation ---

func TestDocumentService_BatchOperation_Upload(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")

	docs := []map[string]interface{}{
		{"@search.action": "upload", "id": "1", "title": "a"},
		{"@search.action": "upload", "id": "2", "title": "b"},
	}
	results, err := svc.BatchOperation(context.Background(), "idx", docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r["status"] != true {
			t.Errorf("expected status=true, got %v", r)
		}
	}
	if n, _ := docRepo.Count("idx"); n != 2 {
		t.Errorf("expected 2 docs persisted, got %d", n)
	}
}

func TestDocumentService_BatchOperation_MergeOrUpload(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")

	docs := []map[string]interface{}{
		{"@search.action": "mergeOrUpload", "id": "1", "title": "x"},
	}
	results, err := svc.BatchOperation(context.Background(), "idx", docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0]["status"] != true {
		t.Errorf("expected success, got %v", results[0])
	}
	if _, err := docRepo.Find("idx", "1"); err != nil {
		t.Errorf("doc should be persisted: %v", err)
	}
}

func TestDocumentService_BatchOperation_MergeExisting(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	_ = docRepo.Upsert(&domain.Document{
		IndexName: "idx", Key: "1",
		Content: `{"id":"1","title":"old","author":"alice"}`,
	})

	docs := []map[string]interface{}{
		{"@search.action": "merge", "id": "1", "title": "new"},
	}
	results, err := svc.BatchOperation(context.Background(), "idx", docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0]["status"] != true {
		t.Fatalf("expected success, got %v", results[0])
	}

	got, _ := docRepo.Find("idx", "1")
	var content map[string]interface{}
	_ = json.Unmarshal([]byte(got.Content), &content)
	if content["title"] != "new" {
		t.Errorf("title not merged, got %v", content["title"])
	}
	if content["author"] != "alice" {
		t.Errorf("author should be retained, got %v", content["author"])
	}
	if content["id"] != "1" {
		t.Errorf("id should be retained, got %v", content["id"])
	}
}

func TestDocumentService_BatchOperation_MergeMissingDoc(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")

	docs := []map[string]interface{}{
		{"@search.action": "merge", "id": "ghost", "title": "n"},
	}
	results, err := svc.BatchOperation(context.Background(), "idx", docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0]["status"] != false {
		t.Errorf("expected failure for missing doc merge, got %v", results[0])
	}
	if results[0]["error"] != "Not found for merge" {
		t.Errorf("unexpected error message: %v", results[0]["error"])
	}
}

func TestDocumentService_BatchOperation_Delete(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1"}`})

	docs := []map[string]interface{}{
		{"@search.action": "delete", "id": "1"},
	}
	results, err := svc.BatchOperation(context.Background(), "idx", docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0]["status"] != true {
		t.Errorf("expected success, got %v", results[0])
	}
	if _, err := docRepo.Find("idx", "1"); err == nil {
		t.Errorf("doc should be deleted")
	}
}

func TestDocumentService_BatchOperation_MissingAction(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")

	results, err := svc.BatchOperation(context.Background(), "idx", []map[string]interface{}{
		{"id": "1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0]["status"] != false || results[0]["error"] != "Missing @search.action" {
		t.Errorf("unexpected result: %v", results[0])
	}
}

func TestDocumentService_BatchOperation_MissingKey(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")

	results, err := svc.BatchOperation(context.Background(), "idx", []map[string]interface{}{
		{"@search.action": "upload", "title": "no key"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0]["status"] != false || results[0]["error"] != "Missing key field" {
		t.Errorf("unexpected result: %v", results[0])
	}
}

func TestDocumentService_BatchOperation_NonStringKey(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")

	results, err := svc.BatchOperation(context.Background(), "idx", []map[string]interface{}{
		{"@search.action": "upload", "id": 99},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0]["error"] != "Key field must be string" {
		t.Errorf("unexpected result: %v", results[0])
	}
}

func TestDocumentService_BatchOperation_UnknownAction(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")

	results, err := svc.BatchOperation(context.Background(), "idx", []map[string]interface{}{
		{"@search.action": "frobnicate", "id": "1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0]["error"] != "Unknown action" {
		t.Errorf("expected unknown action error, got %v", results[0])
	}
}

func TestDocumentService_BatchOperation_IndexNotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newDocumentServiceForTest()
	_, err := svc.BatchOperation(context.Background(), "missing", []map[string]interface{}{})
	if err == nil || err.Error() != "index not found" {
		t.Fatalf("expected 'index not found', got %v", err)
	}
}

func TestDocumentService_BatchOperation_EmptyDocs(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")

	results, err := svc.BatchOperation(context.Background(), "idx", []map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestDocumentService_BatchOperation_BadSchema(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	idxRepo.store["bad"] = &domain.Index{Name: "bad", Schema: "garbage"}

	_, err := svc.BatchOperation(context.Background(), "bad", []map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "schema parse error") {
		t.Fatalf("expected schema parse error, got %v", err)
	}
}

// --- SearchDocuments ---

func TestDocumentService_SearchDocuments_NoFilter(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1","title":"alpha"}`})
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "2", Content: `{"id":"2","title":"beta"}`})

	results, err := svc.SearchDocuments(context.Background(), "idx", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestDocumentService_SearchDocuments_Wildcard(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1"}`})

	results, err := svc.SearchDocuments(context.Background(), "idx", "*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestDocumentService_SearchDocuments_CaseInsensitive(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1","title":"Alpha"}`})
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "2", Content: `{"id":"2","title":"Beta"}`})

	results, err := svc.SearchDocuments(context.Background(), "idx", "ALPHA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0]["title"] != "Alpha" {
		t.Errorf("expected title=Alpha, got %v", results[0]["title"])
	}
}

func TestDocumentService_SearchDocuments_NoMatch(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1","title":"alpha"}`})

	results, err := svc.SearchDocuments(context.Background(), "idx", "zzz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestDocumentService_SearchDocuments_IndexNotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newDocumentServiceForTest()
	_, err := svc.SearchDocuments(context.Background(), "missing", "x")
	if err == nil || err.Error() != "index not found" {
		t.Fatalf("expected 'index not found', got %v", err)
	}
}

func TestDocumentService_SearchDocuments_SkipsBadJSON(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	// Insert raw broken JSON via the mock store directly.
	docRepo.store["idx"] = map[string]*domain.Document{
		"bad":  {IndexName: "idx", Key: "bad", Content: "not-json"},
		"good": {IndexName: "idx", Key: "good", Content: `{"id":"good"}`},
	}
	results, err := svc.SearchDocuments(context.Background(), "idx", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 valid result (bad JSON skipped), got %d", len(results))
	}
}

// --- contains helper ---

func TestContainsHelper(t *testing.T) {
	t.Parallel()
	cases := []struct {
		content string
		search  string
		want    bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello", "xyz", false},
		{"anything", "", true},
		{"anything", "*", true},
	}
	for _, c := range cases {
		if got := contains(c.content, c.search); got != c.want {
			t.Errorf("contains(%q, %q) = %v, want %v", c.content, c.search, got, c.want)
		}
	}
}

// --- GetDocument ---

func TestDocumentService_GetDocument_Success(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1","title":"hi"}`})

	got, err := svc.GetDocument(context.Background(), "idx", "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["title"] != "hi" {
		t.Errorf("expected title=hi, got %v", got["title"])
	}
}

func TestDocumentService_GetDocument_IndexNotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newDocumentServiceForTest()
	_, err := svc.GetDocument(context.Background(), "missing", "1")
	if err == nil || err.Error() != "index not found" {
		t.Fatalf("expected 'index not found', got %v", err)
	}
}

func TestDocumentService_GetDocument_DocNotFound(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	_, err := svc.GetDocument(context.Background(), "idx", "ghost")
	if err == nil || err.Error() != "document not found" {
		t.Fatalf("expected 'document not found', got %v", err)
	}
}

func TestDocumentService_GetDocument_BadStoredJSON(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	docRepo.store["idx"] = map[string]*domain.Document{
		"1": {IndexName: "idx", Key: "1", Content: "not-json"},
	}
	_, err := svc.GetDocument(context.Background(), "idx", "1")
	if err == nil || !strings.Contains(err.Error(), "invalid document json") {
		t.Fatalf("expected 'invalid document json', got %v", err)
	}
}

// --- CountDocuments ---

func TestDocumentService_CountDocuments_Success(t *testing.T) {
	t.Parallel()
	svc, idxRepo, docRepo := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1"}`})
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "2", Content: `{"id":"2"}`})
	_ = docRepo.Upsert(&domain.Document{IndexName: "idx", Key: "3", Content: `{"id":"3"}`})

	got, err := svc.CountDocuments(context.Background(), "idx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 3 {
		t.Errorf("count = %d, want 3", got)
	}
}

func TestDocumentService_CountDocuments_IndexNotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newDocumentServiceForTest()
	_, err := svc.CountDocuments(context.Background(), "missing")
	if err == nil || err.Error() != "index not found" {
		t.Fatalf("expected 'index not found', got %v", err)
	}
}

func TestDocumentService_CountDocuments_EmptyIndex(t *testing.T) {
	t.Parallel()
	svc, idxRepo, _ := newDocumentServiceForTest()
	seedIndex(t, idxRepo, "idx")
	got, err := svc.CountDocuments(context.Background(), "idx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("count = %d, want 0", got)
	}
}
