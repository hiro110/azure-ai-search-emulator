package application

import (
	"ai-search-emulator/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type DocumentService struct {
	DocRepo domain.DocumentRepository
	IdxRepo domain.IndexRepository
}

func NewDocumentService(docRepo domain.DocumentRepository, idxRepo domain.IndexRepository) *DocumentService {
	return &DocumentService{DocRepo: docRepo, IdxRepo: idxRepo}
}

func (s *DocumentService) AddOrUpdateSingleDoc(ctx context.Context, indexName string, doc map[string]interface{}) error {
	exists, err := s.IdxRepo.Exists(indexName)
	if err != nil {
		return err
	}
	if !exists {
		return domain.ErrIndexNotFound
	}
	keyField, err := s.keyField(indexName)
	if err != nil {
		return err
	}
	keyVal, ok := doc[keyField]
	if !ok {
		return domain.ErrMissingKeyField
	}
	keyStr, ok := keyVal.(string)
	if !ok {
		return fmt.Errorf("key field must be string")
	}
	docJSON, _ := json.Marshal(doc)
	return s.DocRepo.Upsert(&domain.Document{
		IndexName: indexName,
		Key:       keyStr,
		Content:   string(docJSON),
	})
}

func (s *DocumentService) BatchOperation(ctx context.Context, indexName string, docs []map[string]interface{}) ([]map[string]interface{}, error) {
	exists, err := s.IdxRepo.Exists(indexName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrIndexNotFound
	}
	keyField, err := s.keyField(indexName)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, len(docs))
	for _, d := range docs {
		action, ok := d["@search.action"].(string)
		if !ok {
			results = append(results, batchError("", http.StatusBadRequest, "Missing @search.action"))
			continue
		}
		keyVal, ok := d[keyField]
		if !ok {
			results = append(results, batchError("", http.StatusBadRequest, "Missing key field"))
			continue
		}
		keyStr, ok := keyVal.(string)
		if !ok {
			results = append(results, batchError("", http.StatusBadRequest, "Key field must be a string"))
			continue
		}
		docJSON, _ := json.Marshal(d)

		switch action {
		case "upload":
			_, findErr := s.DocRepo.Find(indexName, keyStr)
			isNew := errors.Is(findErr, domain.ErrDocumentNotFound)
			if err := s.DocRepo.Upsert(&domain.Document{IndexName: indexName, Key: keyStr, Content: string(docJSON)}); err != nil {
				results = append(results, batchError(keyStr, http.StatusInternalServerError, err.Error()))
			} else if isNew {
				results = append(results, batchSuccess(keyStr, http.StatusCreated))
			} else {
				results = append(results, batchSuccess(keyStr, http.StatusOK))
			}
		case "mergeOrUpload":
			_, findErr := s.DocRepo.Find(indexName, keyStr)
			isNew := errors.Is(findErr, domain.ErrDocumentNotFound)
			if err := s.DocRepo.Upsert(&domain.Document{IndexName: indexName, Key: keyStr, Content: string(docJSON)}); err != nil {
				results = append(results, batchError(keyStr, http.StatusInternalServerError, err.Error()))
			} else if isNew {
				results = append(results, batchSuccess(keyStr, http.StatusCreated))
			} else {
				results = append(results, batchSuccess(keyStr, http.StatusOK))
			}
		case "merge":
			old, err := s.DocRepo.Find(indexName, keyStr)
			if err != nil {
				results = append(results, batchError(keyStr, http.StatusNotFound, "Document not found for merge"))
				continue
			}
			var oldDoc map[string]interface{}
			_ = json.Unmarshal([]byte(old.Content), &oldDoc)
			for k, v := range d {
				if k != "@search.action" && k != keyField {
					oldDoc[k] = v
				}
			}
			mergedJSON, _ := json.Marshal(oldDoc)
			if err := s.DocRepo.Upsert(&domain.Document{IndexName: indexName, Key: keyStr, Content: string(mergedJSON)}); err != nil {
				results = append(results, batchError(keyStr, http.StatusInternalServerError, err.Error()))
			} else {
				results = append(results, batchSuccess(keyStr, http.StatusOK))
			}
		case "delete":
			if err := s.DocRepo.Delete(indexName, keyStr); err != nil {
				results = append(results, batchError(keyStr, http.StatusInternalServerError, err.Error()))
			} else {
				results = append(results, batchSuccess(keyStr, http.StatusOK))
			}
		default:
			results = append(results, batchError(keyStr, http.StatusBadRequest, "Unknown action: "+action))
		}
	}
	return results, nil
}

func batchSuccess(key string, statusCode int) map[string]interface{} {
	return map[string]interface{}{
		"key":        key,
		"status":     true,
		"statusCode": statusCode,
	}
}

func batchError(key string, statusCode int, message string) map[string]interface{} {
	r := map[string]interface{}{
		"status":       false,
		"statusCode":   statusCode,
		"errorMessage": message,
	}
	if key != "" {
		r["key"] = key
	}
	return r
}

// SearchDocuments executes a search using the provided OData parameters.
func (s *DocumentService) SearchDocuments(ctx context.Context, indexName string, params SearchParams) (*SearchResult, error) {
	exists, err := s.IdxRepo.Exists(indexName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrIndexNotFound
	}

	opts := domain.SearchOptions{
		TextSearch:       params.Search,
		TextSearchFields: params.SearchFields,
		Top:              params.Top,
		Skip:             params.Skip,
	}
	if opts.Top <= 0 {
		opts.Top = defaultTop
	}

	if params.Filter != "" {
		whereSQL, whereArgs, err := ParseODataFilter(params.Filter)
		if err != nil {
			return nil, fmt.Errorf("invalid $filter: %w", err)
		}
		opts.WhereSQL = whereSQL
		opts.WhereArgs = whereArgs
	}

	if params.OrderBy != "" {
		orderSQL, err := ParseODataOrderBy(params.OrderBy)
		if err != nil {
			return nil, fmt.Errorf("invalid $orderby: %w", err)
		}
		opts.OrderSQL = orderSQL
	}

	docs, total, err := s.DocRepo.Search(indexName, opts)
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(doc.Content), &m); err == nil {
			if len(params.Select) > 0 {
				m = selectFields(m, params.Select)
			}
			results = append(results, m)
		}
	}

	return &SearchResult{Value: results, Total: total}, nil
}

func (s *DocumentService) GetDocument(ctx context.Context, indexName, key string) (map[string]interface{}, error) {
	exists, err := s.IdxRepo.Exists(indexName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrIndexNotFound
	}
	doc, err := s.DocRepo.Find(indexName, key)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(doc.Content), &m); err != nil {
		return nil, fmt.Errorf("invalid document json")
	}
	return m, nil
}

func (s *DocumentService) CountDocuments(ctx context.Context, indexName string) (int, error) {
	exists, err := s.IdxRepo.Exists(indexName)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, domain.ErrIndexNotFound
	}
	return s.DocRepo.Count(indexName)
}

// keyField extracts the name of the key field from the index schema.
func (s *DocumentService) keyField(indexName string) (string, error) {
	idx, err := s.IdxRepo.FindByName(indexName)
	if err != nil {
		return "", err
	}
	var schema struct {
		Fields []struct {
			Name string `json:"name"`
			Key  bool   `json:"key"`
		}
	}
	if err := json.Unmarshal([]byte(idx.Schema), &schema); err != nil {
		return "", fmt.Errorf("schema parse error")
	}
	for _, f := range schema.Fields {
		if f.Key {
			return f.Name, nil
		}
	}
	return "", domain.ErrMissingKeyField
}

// selectFields returns a new map containing only the requested fields.
func selectFields(m map[string]interface{}, fields []string) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		if v, ok := m[f]; ok {
			result[f] = v
		}
	}
	return result
}

// contains reports whether content includes the search term (case-insensitive).
func contains(content string, search string) bool {
	if search == "" {
		return true
	}
	if search == "*" {
		return true
	}
	return strings.Contains(strings.ToLower(content), strings.ToLower(search))
}
