package infrastructure

import (
	"errors"
	"fmt"
	"testing"

	"ai-search-emulator/internal/domain"
)

// seedTestIndex inserts an index row so foreign key constraints (if enabled)
// won't reject document inserts and so the test data is realistic.
func seedTestIndex(t *testing.T, repo *SQLiteIndexRepository, name string) {
	t.Helper()
	if err := repo.Create(&domain.Index{Name: name, Schema: "{}"}); err != nil {
		t.Fatalf("failed to seed index: %v", err)
	}
}

func TestSQLiteDocumentRepository_Upsert_Insert(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	doc := &domain.Document{IndexName: "idx", Key: "k1", Content: `{"id":"k1"}`}
	if err := repo.Upsert(doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := repo.Find("idx", "k1")
	if err != nil {
		t.Fatalf("expected to find inserted doc: %v", err)
	}
	if got.Content != `{"id":"k1"}` {
		t.Errorf("content = %q", got.Content)
	}
}

func TestSQLiteDocumentRepository_Upsert_ReplaceExisting(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "k1", Content: "old"})
	if err := repo.Upsert(&domain.Document{IndexName: "idx", Key: "k1", Content: "new"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := repo.Find("idx", "k1")
	if got.Content != "new" {
		t.Errorf("content not replaced, got %q", got.Content)
	}
	count, _ := repo.Count("idx")
	if count != 1 {
		t.Errorf("expected 1 row after upsert, got %d", count)
	}
}

func TestSQLiteDocumentRepository_Find_NotFoundReturnsSentinel(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	_, err := repo.Find("idx", "missing")
	if !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Errorf("expected ErrDocumentNotFound, got %v", err)
	}
}

func TestSQLiteDocumentRepository_Delete(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)
	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "k1", Content: "v"})

	if err := repo.Delete("idx", "k1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := repo.Find("idx", "k1"); !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Errorf("expected doc deleted, got %v", err)
	}
}

func TestSQLiteDocumentRepository_Delete_MissingIsNoOp(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	// Delete on missing key currently returns nil (no rows-affected check).
	// Document this current contract so a regression is caught.
	if err := repo.Delete("idx", "ghost"); err != nil {
		t.Errorf("expected nil for missing key delete, got %v", err)
	}
}

func TestSQLiteDocumentRepository_List(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "a")
	seedTestIndex(t, idxRepo, "b")
	repo := NewSQLiteDocumentRepository(db)

	_ = repo.Upsert(&domain.Document{IndexName: "a", Key: "1", Content: "x"})
	_ = repo.Upsert(&domain.Document{IndexName: "a", Key: "2", Content: "y"})
	_ = repo.Upsert(&domain.Document{IndexName: "b", Key: "1", Content: "z"})

	gotA, err := repo.List("a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotA) != 2 {
		t.Errorf("expected 2 docs in 'a', got %d", len(gotA))
	}
	gotB, _ := repo.List("b")
	if len(gotB) != 1 {
		t.Errorf("expected 1 doc in 'b', got %d", len(gotB))
	}
	gotEmpty, _ := repo.List("c")
	if len(gotEmpty) != 0 {
		t.Errorf("expected empty list for unknown index, got %d", len(gotEmpty))
	}
}

func TestSQLiteDocumentRepository_Count(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	if c, err := repo.Count("idx"); err != nil || c != 0 {
		t.Errorf("expected 0,nil — got %d,%v", c, err)
	}
	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: "x"})
	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "2", Content: "y"})
	if c, err := repo.Count("idx"); err != nil || c != 2 {
		t.Errorf("expected 2,nil — got %d,%v", c, err)
	}
}

func TestSQLiteDocumentRepository_OperationsOnClosedDB(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteDocumentRepository(db)
	_ = db.Close()

	if err := repo.Upsert(&domain.Document{IndexName: "x", Key: "1", Content: "v"}); err == nil {
		t.Errorf("expected error on closed db (Upsert)")
	}
	if _, err := repo.Find("x", "1"); err == nil {
		t.Errorf("expected error on closed db (Find)")
	}
	if err := repo.Delete("x", "1"); err == nil {
		t.Errorf("expected error on closed db (Delete)")
	}
	if _, err := repo.List("x"); err == nil {
		t.Errorf("expected error on closed db (List)")
	}
	if _, err := repo.Count("x"); err == nil {
		t.Errorf("expected error on closed db (Count)")
	}
}

// --- Search ---

func TestSQLiteDocumentRepository_Search_NoFilter(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	for i := 1; i <= 3; i++ {
		_ = repo.Upsert(&domain.Document{
			IndexName: "idx", Key: fmt.Sprintf("%d", i),
			Content: fmt.Sprintf(`{"id":"%d","title":"doc%d"}`, i, i),
		})
	}

	docs, total, err := repo.Search("idx", domain.SearchOptions{Top: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(docs) != 3 {
		t.Errorf("len(docs) = %d, want 3", len(docs))
	}
}

func TestSQLiteDocumentRepository_Search_TextSearch(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1","title":"Alpha Hotel"}`})
	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "2", Content: `{"id":"2","title":"Beta Motel"}`})

	docs, total, err := repo.Search("idx", domain.SearchOptions{TextSearch: "HOTEL", Top: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(docs) != 1 || docs[0].Key != "1" {
		t.Errorf("unexpected result: %v", docs)
	}
}

func TestSQLiteDocumentRepository_Search_TextSearchFields(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1","title":"Alpha","notes":"Hotel nearby"}`})
	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "2", Content: `{"id":"2","title":"Hotel","notes":"nothing"}`})

	// Search only in 'title' field — doc 1 has "Alpha" in title, not "hotel".
	// doc 2 has "Hotel" in title.
	docs, total, err := repo.Search("idx", domain.SearchOptions{
		TextSearch:       "hotel",
		TextSearchFields: []string{"title"},
		Top:              10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || docs[0].Key != "2" {
		t.Errorf("expected only doc2, got total=%d docs=%v", total, docs)
	}
}

func TestSQLiteDocumentRepository_Search_ODataFilter(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "1", Content: `{"id":"1","Category":"Hotel","Rating":5}`})
	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "2", Content: `{"id":"2","Category":"Motel","Rating":3}`})
	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "3", Content: `{"id":"3","Category":"Hotel","Rating":2}`})

	docs, total, err := repo.Search("idx", domain.SearchOptions{
		WhereSQL:  "json_extract(content, '$.Category') = ? AND json_extract(content, '$.Rating') >= ?",
		WhereArgs: []interface{}{"Hotel", int64(4)},
		Top:       10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || docs[0].Key != "1" {
		t.Errorf("expected doc1, got total=%d docs=%v", total, docs)
	}
}

func TestSQLiteDocumentRepository_Search_TopSkip(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	for i := 1; i <= 5; i++ {
		_ = repo.Upsert(&domain.Document{
			IndexName: "idx", Key: fmt.Sprintf("%d", i),
			Content: fmt.Sprintf(`{"id":"%d"}`, i),
		})
	}

	docs, total, err := repo.Search("idx", domain.SearchOptions{Top: 2, Skip: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(docs) != 2 {
		t.Errorf("len(docs) = %d, want 2", len(docs))
	}
}

func TestSQLiteDocumentRepository_Search_DefaultTop(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	for i := 1; i <= 60; i++ {
		_ = repo.Upsert(&domain.Document{
			IndexName: "idx", Key: fmt.Sprintf("%d", i),
			Content: fmt.Sprintf(`{"id":"%d"}`, i),
		})
	}

	docs, total, err := repo.Search("idx", domain.SearchOptions{Top: 0}) // 0 = use default (50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 60 {
		t.Errorf("total = %d, want 60", total)
	}
	if len(docs) != 50 {
		t.Errorf("len(docs) = %d, want 50 (default top)", len(docs))
	}
}

func TestSQLiteDocumentRepository_Search_OrderBy(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	idxRepo := NewSQLiteIndexRepository(db)
	seedTestIndex(t, idxRepo, "idx")
	repo := NewSQLiteDocumentRepository(db)

	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "a", Content: `{"id":"a","Rating":1}`})
	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "b", Content: `{"id":"b","Rating":3}`})
	_ = repo.Upsert(&domain.Document{IndexName: "idx", Key: "c", Content: `{"id":"c","Rating":2}`})

	docs, _, err := repo.Search("idx", domain.SearchOptions{
		OrderSQL: "json_extract(content, '$.Rating') DESC",
		Top:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 3 || docs[0].Key != "b" || docs[2].Key != "a" {
		t.Errorf("unexpected order: %v", docs)
	}
}
