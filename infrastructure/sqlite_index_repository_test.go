package infrastructure

import (
	"errors"
	"testing"

	"ai-search-emulator/domain"
)

func TestSQLiteIndexRepository_Create_Success(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)

	if err := repo.Create(&domain.Index{Name: "idx", Schema: `{"fields":[]}`}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := repo.FindByName("idx")
	if err != nil {
		t.Fatalf("expected to find created index: %v", err)
	}
	if got.Name != "idx" {
		t.Errorf("name = %q, want idx", got.Name)
	}
	if got.Schema != `{"fields":[]}` {
		t.Errorf("schema = %q", got.Schema)
	}
}

func TestSQLiteIndexRepository_Create_DuplicateFails(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)

	if err := repo.Create(&domain.Index{Name: "idx", Schema: "{}"}); err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}
	err := repo.Create(&domain.Index{Name: "idx", Schema: "{}"})
	if err == nil {
		t.Fatalf("expected duplicate-key error, got nil")
	}
}

func TestSQLiteIndexRepository_Update_Success(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)
	_ = repo.Create(&domain.Index{Name: "idx", Schema: "old"})

	if err := repo.Update(&domain.Index{Name: "idx", Schema: "new"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := repo.FindByName("idx")
	if got.Schema != "new" {
		t.Errorf("schema not updated, got %q", got.Schema)
	}
}

func TestSQLiteIndexRepository_Update_NotFoundReturnsSentinel(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)
	err := repo.Update(&domain.Index{Name: "missing", Schema: "x"})
	if !errors.Is(err, domain.ErrIndexNotFound) {
		t.Errorf("expected ErrIndexNotFound, got %v", err)
	}
}

func TestSQLiteIndexRepository_Exists(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)

	exists, err := repo.Exists("idx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Errorf("expected exists=false for unknown index")
	}

	_ = repo.Create(&domain.Index{Name: "idx", Schema: "{}"})
	exists, err = repo.Exists("idx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Errorf("expected exists=true after create")
	}
}

func TestSQLiteIndexRepository_FindByName_NotFoundReturnsSentinel(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)
	_, err := repo.FindByName("missing")
	if !errors.Is(err, domain.ErrIndexNotFound) {
		t.Errorf("expected ErrIndexNotFound, got %v", err)
	}
}

func TestSQLiteIndexRepository_List(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)

	got, err := repo.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty list, got %d", len(got))
	}

	_ = repo.Create(&domain.Index{Name: "a", Schema: "{}"})
	_ = repo.Create(&domain.Index{Name: "b", Schema: "{}"})

	got, err = repo.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
}

func TestSQLiteIndexRepository_Delete_Success(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)
	_ = repo.Create(&domain.Index{Name: "idx", Schema: "{}"})

	if err := repo.Delete("idx"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := repo.FindByName("idx"); !errors.Is(err, domain.ErrIndexNotFound) {
		t.Errorf("expected ErrIndexNotFound after delete, got %v", err)
	}
}

func TestSQLiteIndexRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)
	if err := repo.Delete("missing"); !errors.Is(err, domain.ErrIndexNotFound) {
		t.Errorf("expected ErrIndexNotFound, got %v", err)
	}
}

func TestSQLiteIndexRepository_OperationsOnClosedDB(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := NewSQLiteIndexRepository(db)
	// Close the DB to trigger error paths in Exec/Query.
	_ = db.Close()

	if err := repo.Create(&domain.Index{Name: "x", Schema: "{}"}); err == nil {
		t.Errorf("expected error on closed db (Create)")
	}
	if _, err := repo.FindByName("x"); err == nil {
		t.Errorf("expected error on closed db (FindByName)")
	}
	if _, err := repo.List(); err == nil {
		t.Errorf("expected error on closed db (List)")
	}
	if _, err := repo.Exists("x"); err == nil {
		t.Errorf("expected error on closed db (Exists)")
	}
	if err := repo.Update(&domain.Index{Name: "x", Schema: "y"}); err == nil {
		t.Errorf("expected error on closed db (Update)")
	}
	if err := repo.Delete("x"); err == nil {
		t.Errorf("expected error on closed db (Delete)")
	}
}
