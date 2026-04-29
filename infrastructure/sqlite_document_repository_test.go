package infrastructure

import (
	"errors"
	"testing"

	"ai-search-emulator/domain"
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
