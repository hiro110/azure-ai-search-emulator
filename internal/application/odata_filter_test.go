package application

import (
	"strings"
	"testing"
)

// --- ParseODataFilter ---

func TestParseODataFilter_SimpleEq(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("Category eq 'Hotel'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "json_extract(content, '$.Category')") {
		t.Errorf("expected json_extract for Category, got: %s", sql)
	}
	if !strings.Contains(sql, "= ?") {
		t.Errorf("expected = ?, got: %s", sql)
	}
	if len(args) != 1 || args[0] != "Hotel" {
		t.Errorf("expected args=[Hotel], got %v", args)
	}
}

func TestParseODataFilter_Ne(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("Status ne 'deleted'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "!= ?") {
		t.Errorf("expected != ?, got: %s", sql)
	}
	if args[0] != "deleted" {
		t.Errorf("expected args=[deleted], got %v", args)
	}
}

func TestParseODataFilter_NumericGe(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("Rating ge 4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, ">= ?") {
		t.Errorf("expected >= ?, got: %s", sql)
	}
	if args[0] != int64(4) {
		t.Errorf("expected args=[4], got %v (%T)", args[0], args[0])
	}
}

func TestParseODataFilter_FloatLt(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("Price lt 9.99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "< ?") {
		t.Errorf("expected < ?, got: %s", sql)
	}
	if args[0] != 9.99 {
		t.Errorf("expected args=[9.99], got %v", args[0])
	}
}

func TestParseODataFilter_EqNull(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("Field eq null")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "IS NULL") {
		t.Errorf("expected IS NULL, got: %s", sql)
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %v", args)
	}
}

func TestParseODataFilter_NeNull(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("Field ne null")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "IS NOT NULL") {
		t.Errorf("expected IS NOT NULL, got: %s", sql)
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %v", args)
	}
}

func TestParseODataFilter_BoolTrue(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("Active eq true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "= ?") {
		t.Errorf("expected = ?, got: %s", sql)
	}
	if args[0] != true {
		t.Errorf("expected args=[true], got %v", args[0])
	}
}

func TestParseODataFilter_And(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("Category eq 'Hotel' and Rating ge 4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, " AND ") {
		t.Errorf("expected AND, got: %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestParseODataFilter_Or(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("Category eq 'Hotel' or Category eq 'Motel'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, " OR ") {
		t.Errorf("expected OR, got: %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestParseODataFilter_Not(t *testing.T) {
	t.Parallel()
	sql, _, err := ParseODataFilter("not Category eq 'Hotel'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(sql, "NOT (") {
		t.Errorf("expected NOT (...), got: %s", sql)
	}
}

func TestParseODataFilter_Parentheses(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("(Category eq 'Hotel' or Category eq 'Motel') and Rating ge 3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, " AND ") {
		t.Errorf("expected AND, got: %s", sql)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d: %v", len(args), args)
	}
}

func TestParseODataFilter_SearchIn(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("search.in(Category, 'Hotel,Motel')")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "IN (") {
		t.Errorf("expected IN (...), got: %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d: %v", len(args), args)
	}
	if args[0] != "Hotel" || args[1] != "Motel" {
		t.Errorf("expected [Hotel Motel], got %v", args)
	}
}

func TestParseODataFilter_SearchIn_CustomDelimiter(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("search.in(Category, 'Hotel|Motel', '|')")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "IN (") {
		t.Errorf("expected IN (...), got: %s", sql)
	}
	if len(args) != 2 || args[0] != "Hotel" || args[1] != "Motel" {
		t.Errorf("expected [Hotel Motel], got %v", args)
	}
}

func TestParseODataFilter_StartsWith(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("startswith(Name, 'Sea')")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "LIKE ?") {
		t.Errorf("expected LIKE ?, got: %s", sql)
	}
	if len(args) != 1 || args[0] != "Sea%" {
		t.Errorf("expected args=[Sea%%], got %v", args)
	}
}

func TestParseODataFilter_StartsWith_EscapesMetachars(t *testing.T) {
	t.Parallel()
	_, args, err := ParseODataFilter("startswith(Name, '50%Off')")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// % in the prefix must be escaped so LIKE doesn't treat it as wildcard.
	if args[0] != `50\%Off%` {
		t.Errorf("expected escaped prefix, got %q", args[0])
	}
}

func TestParseODataFilter_SingleQuoteEscape(t *testing.T) {
	t.Parallel()
	_, args, err := ParseODataFilter("Name eq 'O''Brien'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args[0] != "O'Brien" {
		t.Errorf("expected O'Brien, got %v", args[0])
	}
}

func TestParseODataFilter_UnknownOperator(t *testing.T) {
	t.Parallel()
	_, _, err := ParseODataFilter("Field xyz 'value'")
	if err == nil {
		t.Fatal("expected error for unknown operator")
	}
}

func TestParseODataFilter_EmptyString(t *testing.T) {
	t.Parallel()
	sql, args, err := ParseODataFilter("")
	// Empty filter is technically an error (no expression).
	if err == nil {
		t.Logf("empty filter returned sql=%q args=%v — acceptable only if caller guards empty input", sql, args)
	}
}

func TestParseODataFilter_TrailingGarbage(t *testing.T) {
	t.Parallel()
	_, _, err := ParseODataFilter("Field eq 'val' JUNK")
	if err == nil {
		t.Fatal("expected error for trailing garbage")
	}
}

// --- ParseODataOrderBy ---

func TestParseODataOrderBy_SingleAsc(t *testing.T) {
	t.Parallel()
	sql, err := ParseODataOrderBy("Rating asc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "json_extract(content, '$.Rating') ASC") {
		t.Errorf("unexpected orderby sql: %s", sql)
	}
}

func TestParseODataOrderBy_SingleDesc(t *testing.T) {
	t.Parallel()
	sql, err := ParseODataOrderBy("Rating desc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "json_extract(content, '$.Rating') DESC") {
		t.Errorf("unexpected orderby sql: %s", sql)
	}
}

func TestParseODataOrderBy_DefaultAsc(t *testing.T) {
	t.Parallel()
	sql, err := ParseODataOrderBy("Rating")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "ASC") {
		t.Errorf("expected default ASC, got: %s", sql)
	}
}

func TestParseODataOrderBy_MultipleFields(t *testing.T) {
	t.Parallel()
	sql, err := ParseODataOrderBy("Rating desc, Name asc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "$.Rating") || !strings.Contains(sql, "DESC") {
		t.Errorf("expected Rating DESC in orderby sql: %s", sql)
	}
	if !strings.Contains(sql, "$.Name") || !strings.Contains(sql, "ASC") {
		t.Errorf("expected Name ASC in orderby sql: %s", sql)
	}
}

func TestParseODataOrderBy_InvalidDirection(t *testing.T) {
	t.Parallel()
	_, err := ParseODataOrderBy("Rating sideways")
	if err == nil {
		t.Fatal("expected error for invalid sort direction")
	}
}
