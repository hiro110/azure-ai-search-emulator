package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"ai-search-emulator/internal/application"
	"ai-search-emulator/internal/infrastructure"
)

const apiTestKey = "test-api-key"

const apiTestSchema = `{
	"name": "movies",
	"fields": [
		{"name": "id", "type": "Edm.String", "key": true},
		{"name": "title", "type": "Edm.String"}
	]
}`

const apiTestSchemaSQL = `
CREATE TABLE IF NOT EXISTS indexes (
    name TEXT PRIMARY KEY,
    schema TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS documents (
    index_name TEXT NOT NULL,
    key TEXT NOT NULL,
    content TEXT NOT NULL,
    PRIMARY KEY (index_name, key),
    FOREIGN KEY (index_name) REFERENCES indexes(name) ON DELETE CASCADE
);`

// setupRouter wires up an in-memory SQLite-backed router that mirrors the
// production main.go bootstrap. The DB is closed automatically when the test
// completes so no files are left behind.
func setupRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("API_KEY", apiTestKey)

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(apiTestSchemaSQL); err != nil {
		_ = db.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	idxRepo := infrastructure.NewSQLiteIndexRepository(db)
	docRepo := infrastructure.NewSQLiteDocumentRepository(db)
	apps := &application.AppServices{
		IndexService:    application.NewIndexService(idxRepo, docRepo),
		DocumentService: application.NewDocumentService(docRepo, idxRepo),
	}

	r := gin.New()
	RegisterHealthCheck(r)
	r.Use(ApiKeyAuthMiddleware())
	RegisterRoutes(r, apps)
	return r
}

// doRequest sends a request with the API key header pre-set. Pass an empty
// string for body when there is no payload.
func doRequest(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("api-key", apiTestKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// --- Health check ---

func TestHealthCheck_NoAuthRequired(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %v", body)
	}
}

// --- API key middleware ---

func TestApiKeyMiddleware_MissingKeyRejected(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/indexes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestApiKeyMiddleware_WrongKeyRejected(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/indexes", nil)
	req.Header.Set("api-key", "wrong")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestApiKeyMiddleware_AcceptsCapitalizedHeader(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/indexes", nil)
	// canonical: Api-Key (Go normalizes to Api-Key already).
	req.Header.Set("Api-Key", apiTestKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestApiKeyMiddleware_RejectsAllWhenEnvUnset(t *testing.T) {
	t.Setenv("API_KEY", "")
	// Build router manually so the env var (or lack thereof) is captured by
	// the middleware closure.
	gin.SetMode(gin.TestMode)
	db, _ := sql.Open("sqlite3", ":memory:")
	db.SetMaxOpenConns(1)
	_, _ = db.Exec(apiTestSchemaSQL)
	t.Cleanup(func() { _ = db.Close() })
	idxRepo := infrastructure.NewSQLiteIndexRepository(db)
	docRepo := infrastructure.NewSQLiteDocumentRepository(db)
	apps := &application.AppServices{
		IndexService:    application.NewIndexService(idxRepo, docRepo),
		DocumentService: application.NewDocumentService(docRepo, idxRepo),
	}
	r := gin.New()
	RegisterHealthCheck(r)
	r.Use(ApiKeyAuthMiddleware())
	RegisterRoutes(r, apps)

	// Even providing some api-key should be rejected because env is empty.
	req := httptest.NewRequest(http.MethodGet, "/indexes", nil)
	req.Header.Set("api-key", "anything")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 when API_KEY env not set", rec.Code)
	}
	// /healthz must remain accessible because it is registered before the
	// middleware.
	hReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	hRec := httptest.NewRecorder()
	r.ServeHTTP(hRec, hReq)
	if hRec.Code != http.StatusOK {
		t.Errorf("/healthz should not require auth, got %d", hRec.Code)
	}
}

// --- POST /indexes ---

func TestCreateIndex_Success(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateIndex_InvalidJSONReturns400(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodPost, "/indexes", "not-json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateIndex_DuplicateReturns409(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup failed: %d", rec.Code)
	}
	rec = doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

// --- GET /indexes ---

func TestListIndexes_Empty(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodGet, "/indexes", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if v, ok := body["value"]; ok {
		// nil array marshals to "null"; the value field must exist with len 0.
		if arr, ok := v.([]interface{}); ok && len(arr) != 0 {
			t.Errorf("expected empty array, got %v", arr)
		}
	}
}

func TestListIndexes_WithSelect(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)

	rec := doRequest(t, r, http.MethodGet, "/indexes?$select=name", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Value []map[string]interface{} `json:"value"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Value) != 1 {
		t.Fatalf("expected 1 result, got %d", len(body.Value))
	}
	if _, ok := body.Value[0]["fields"]; ok {
		t.Errorf("fields should be filtered out by $select=name")
	}
}

// --- GET /indexes/:index ---

func TestGetIndex_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)

	rec := doRequest(t, r, http.MethodGet, "/indexes/movies", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["name"] != "movies" {
		t.Errorf("name = %v", body["name"])
	}
}

func TestGetIndex_NotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodGet, "/indexes/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- PUT /indexes/:index ---

func TestUpdateIndex_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)

	updated := `{"name":"movies","fields":[{"name":"id","key":true}]}`
	rec := doRequest(t, r, http.MethodPut, "/indexes/movies", updated)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateIndex_CreatesWhenNotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodPut, "/indexes/missing", apiTestSchema)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201 (PUT creates when index absent)", rec.Code)
	}
}

func TestUpdateIndex_InvalidJSON(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	rec := doRequest(t, r, http.MethodPut, "/indexes/movies", "garbage")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- DELETE /indexes/:index ---

func TestDeleteIndex_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)

	rec := doRequest(t, r, http.MethodDelete, "/indexes/movies", "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestDeleteIndex_NotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodDelete, "/indexes/missing", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- GET /indexes/:index/stats ---

func TestGetIndexStats_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs", `{"id":"1","title":"a"}`)

	rec := doRequest(t, r, http.MethodGet, "/indexes/movies/stats", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if int(body["documentCount"].(float64)) != 1 {
		t.Errorf("documentCount = %v", body["documentCount"])
	}
}

func TestGetIndexStats_NotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodGet, "/indexes/missing/stats", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- POST /indexes/:index/docs ---

func TestAddSingleDoc_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)

	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs", `{"id":"1","title":"hello"}`)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAddSingleDoc_IndexNotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodPost, "/indexes/missing/docs", `{"id":"1"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAddSingleDoc_MissingKey(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs", `{"title":"no key"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAddSingleDoc_InvalidJSON(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs", "not-json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- POST /indexes/:index/docs/index (batch) ---

func TestBatchOperation_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)

	batch := `{"value":[
		{"@search.action":"upload","id":"1","title":"a"},
		{"@search.action":"upload","id":"2","title":"b"}
	]}`
	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index", batch)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Value []map[string]interface{} `json:"value"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Value) != 2 {
		t.Errorf("expected 2 results, got %d", len(body.Value))
	}
	for _, v := range body.Value {
		if v["status"] != true {
			t.Errorf("expected status=true, got %v", v)
		}
		if v["statusCode"] != float64(201) {
			t.Errorf("expected statusCode=201, got %v", v["statusCode"])
		}
	}
}

func TestBatchOperation_PartialFailureReturns207(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)

	// upload one doc first so merge on "1" can succeed
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index",
		`{"value":[{"@search.action":"upload","id":"1","title":"a"}]}`)

	batch := `{"value":[
		{"@search.action":"merge","id":"1","title":"updated"},
		{"@search.action":"merge","id":"nonexistent","title":"x"}
	]}`
	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index", batch)
	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207", rec.Code)
	}
	var body struct {
		Value []map[string]interface{} `json:"value"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Value) != 2 {
		t.Fatalf("expected 2 results, got %d", len(body.Value))
	}
	// first item succeeded
	if body.Value[0]["status"] != true {
		t.Errorf("result[0].status = %v, want true", body.Value[0]["status"])
	}
	if body.Value[0]["statusCode"] != float64(200) {
		t.Errorf("result[0].statusCode = %v, want 200", body.Value[0]["statusCode"])
	}
	// second item failed
	if body.Value[1]["status"] != false {
		t.Errorf("result[1].status = %v, want false", body.Value[1]["status"])
	}
	if body.Value[1]["statusCode"] != float64(404) {
		t.Errorf("result[1].statusCode = %v, want 404", body.Value[1]["statusCode"])
	}
	if body.Value[1]["errorMessage"] == nil {
		t.Errorf("result[1].errorMessage should be set")
	}
}

func TestBatchOperation_MergeOrUploadReturnsCorrectStatusCodes(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index",
		`{"value":[{"@search.action":"upload","id":"1","title":"a"}]}`)

	batch := `{"value":[
		{"@search.action":"mergeOrUpload","id":"1","title":"updated"},
		{"@search.action":"mergeOrUpload","id":"2","title":"new"}
	]}`
	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index", batch)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Value []map[string]interface{} `json:"value"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	// existing doc updated → 200
	if body.Value[0]["statusCode"] != float64(200) {
		t.Errorf("result[0].statusCode = %v, want 200 (updated)", body.Value[0]["statusCode"])
	}
	// new doc created → 201
	if body.Value[1]["statusCode"] != float64(201) {
		t.Errorf("result[1].statusCode = %v, want 201 (created)", body.Value[1]["statusCode"])
	}
}

func TestBatchOperation_DeleteReturns200StatusCode(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index",
		`{"value":[{"@search.action":"upload","id":"1","title":"a"}]}`)

	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index",
		`{"value":[{"@search.action":"delete","id":"1"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Value []map[string]interface{} `json:"value"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Value[0]["statusCode"] != float64(200) {
		t.Errorf("result[0].statusCode = %v, want 200", body.Value[0]["statusCode"])
	}
}

func TestBatchOperation_IndexNotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodPost, "/indexes/missing/docs/index",
		`{"value":[{"@search.action":"upload","id":"1"}]}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestBatchOperation_InvalidBody(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index", "garbage")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- GET /indexes/:index/docs/:key ---

func TestGetDocument_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs", `{"id":"1","title":"hi"}`)

	rec := doRequest(t, r, http.MethodGet, "/indexes/movies/docs/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["title"] != "hi" {
		t.Errorf("title = %v", body["title"])
	}
}

func TestGetDocument_DocNotFound(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	rec := doRequest(t, r, http.MethodGet, "/indexes/movies/docs/ghost", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestGetDocument_IndexNotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodGet, "/indexes/missing/docs/1", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- GET /indexes/:index/docs/$count ---

func TestCountDocuments_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs", `{"id":"1"}`)
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs", `{"id":"2"}`)

	rec := doRequest(t, r, http.MethodGet, "/indexes/movies/docs/$count", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "2" {
		t.Errorf("count body = %q, want '2'", got)
	}
}

func TestCountDocuments_IndexNotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodGet, "/indexes/missing/docs/$count", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- GET /indexes/:index/docs (search) ---

func TestSearchDocuments_GET_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs", `{"id":"1","title":"alpha"}`)
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs", `{"id":"2","title":"beta"}`)

	rec := doRequest(t, r, http.MethodGet, "/indexes/movies/docs?search=alpha", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Value []map[string]interface{} `json:"value"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Value) != 1 {
		t.Errorf("expected 1 result, got %d", len(body.Value))
	}
}

func TestSearchDocuments_GET_IndexNotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodGet, "/indexes/missing/docs", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- POST /indexes/:index/docs/search ---

func TestSearchDocuments_POST_Success(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs", `{"id":"1","title":"alpha"}`)

	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs/search", `{"search":"alpha"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Value []map[string]interface{} `json:"value"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Value) != 1 {
		t.Errorf("expected 1 result, got %d", len(body.Value))
	}
}

func TestSearchDocuments_POST_InvalidBody(t *testing.T) {
	r := setupRouter(t)
	doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	rec := doRequest(t, r, http.MethodPost, "/indexes/movies/docs/search", "not-json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestSearchDocuments_POST_IndexNotFound(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodPost, "/indexes/missing/docs/search", `{"search":"x"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- End-to-end happy path through the full handler stack ---

func TestEndToEnd_CreateAndQuery(t *testing.T) {
	r := setupRouter(t)

	// Create index.
	rec := doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create index failed: %d", rec.Code)
	}

	// Insert two docs via batch.
	batch := `{"value":[
		{"@search.action":"upload","id":"a","title":"foo"},
		{"@search.action":"upload","id":"b","title":"bar"}
	]}`
	rec = doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index", batch)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch failed: %d", rec.Code)
	}

	// Count must be 2.
	rec = doRequest(t, r, http.MethodGet, "/indexes/movies/docs/$count", "")
	if strings.TrimSpace(rec.Body.String()) != "2" {
		t.Errorf("count = %q", rec.Body.String())
	}

	// Merge should preserve fields.
	merge := `{"value":[{"@search.action":"merge","id":"a","title":"FOO"}]}`
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index", merge)
	rec = doRequest(t, r, http.MethodGet, "/indexes/movies/docs/a", "")
	var doc map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &doc)
	if doc["title"] != "FOO" {
		t.Errorf("merge failed: title=%v", doc["title"])
	}

	// Delete one doc.
	del := `{"value":[{"@search.action":"delete","id":"b"}]}`
	doRequest(t, r, http.MethodPost, "/indexes/movies/docs/index", del)
	rec = doRequest(t, r, http.MethodGet, "/indexes/movies/docs/$count", "")
	if strings.TrimSpace(rec.Body.String()) != "1" {
		t.Errorf("count after delete = %q", rec.Body.String())
	}

	// Cleanup: drop the index.
	rec = doRequest(t, r, http.MethodDelete, "/indexes/movies", "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete index = %d", rec.Code)
	}
}

// --- Body re-emission sanity check (CreateIndex echoes the request body) ---

func TestCreateIndex_EchoesRequestBody(t *testing.T) {
	r := setupRouter(t)
	rec := doRequest(t, r, http.MethodPost, "/indexes", apiTestSchema)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	// Body should be the original schema (compact comparison via field).
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"name": "movies"`)) {
		t.Errorf("response body should echo input, got %s", rec.Body.String())
	}
}

// Guard: ensure no DB file is left in cwd after running these tests.
// This is a best-effort assertion — file presence is checked only if it
// somehow gets created in the working directory.
func TestNoDBFileLeak(t *testing.T) {
	// Must run last to confirm no test created a sqlite file in CWD.
	for _, candidate := range []string{"./data.db", "data.db"} {
		if _, err := os.Stat(candidate); err == nil {
			t.Errorf("unexpected DB file left behind: %s", candidate)
		}
	}
}
