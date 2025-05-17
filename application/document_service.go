package application

import (
	"ai-search-emulator/domain"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type DocumentService struct {
	DocRepo   domain.DocumentRepository
	IdxRepo   domain.IndexRepository
}

func NewDocumentService(docRepo domain.DocumentRepository, idxRepo domain.IndexRepository) *DocumentService {
	return &DocumentService{DocRepo: docRepo, IdxRepo: idxRepo}
}

func (s *DocumentService) AddOrUpdateSingleDoc(ctx context.Context, indexName string, doc map[string]interface{}) error {
	// インデックス存在チェック
	exists, err := s.IdxRepo.Exists(indexName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("index not found")
	}
	// キーフィールド名を取得
	idx, err := s.IdxRepo.FindByName(indexName)
	if err != nil {
		return err
	}
	var schema struct {
		Fields []struct {
			Name string `json:"name"`
			Key  bool   `json:"key"`
		}
	}
	if err := json.Unmarshal([]byte(idx.Schema), &schema); err != nil {
		return fmt.Errorf("schema parse error")
	}
	keyField := ""
	for _, f := range schema.Fields {
		if f.Key {
			keyField = f.Name
			break
		}
	}
	if keyField == "" {
		return fmt.Errorf("missing key field")
	}
	keyVal, ok := doc[keyField]
	if !ok {
		return fmt.Errorf("missing key field")
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
		return nil, fmt.Errorf("index not found")
	}
	// キーフィールド名を取得
	idx, err := s.IdxRepo.FindByName(indexName)
	if err != nil {
		return nil, err
	}
	var schema struct {
		Fields []struct {
			Name string `json:"name"`
			Key  bool   `json:"key"`
		}
	}
	if err := json.Unmarshal([]byte(idx.Schema), &schema); err != nil {
		return nil, fmt.Errorf("schema parse error")
	}
	keyField := ""
	for _, f := range schema.Fields {
		if f.Key {
			keyField = f.Name
			break
		}
	}
	if keyField == "" {
		return nil, fmt.Errorf("missing key field")
	}
	results := make([]map[string]interface{}, 0, len(docs))
	for _, d := range docs {
		action, ok := d["@search.action"].(string)
		if !ok {
			results = append(results, map[string]interface{}{"status": false, "error": "Missing @search.action"})
			continue
		}
		keyVal, ok := d[keyField]
		if !ok {
			results = append(results, map[string]interface{}{"status": false, "error": "Missing key field"})
			continue
		}
		keyStr, ok := keyVal.(string)
		if !ok {
			results = append(results, map[string]interface{}{"status": false, "error": "Key field must be string"})
			continue
		}
		docJSON, _ := json.Marshal(d)
		switch action {
		case "upload", "mergeOrUpload":
			err = s.DocRepo.Upsert(&domain.Document{IndexName: indexName, Key: keyStr, Content: string(docJSON)})
			results = append(results, map[string]interface{}{"key": keyStr, "status": err == nil})
		case "merge":
			old, err := s.DocRepo.Find(indexName, keyStr)
			if err != nil {
				results = append(results, map[string]interface{}{"key": keyStr, "status": false, "error": "Not found for merge"})
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
			err = s.DocRepo.Upsert(&domain.Document{IndexName: indexName, Key: keyStr, Content: string(mergedJSON)})
			results = append(results, map[string]interface{}{"key": keyStr, "status": err == nil})
		case "delete":
			err = s.DocRepo.Delete(indexName, keyStr)
			results = append(results, map[string]interface{}{"key": keyStr, "status": err == nil})
		default:
			results = append(results, map[string]interface{}{"key": keyStr, "status": false, "error": "Unknown action"})
		}
	}
	return results, nil
}

func (s *DocumentService) SearchDocuments(ctx context.Context, indexName string, search string) ([]map[string]interface{}, error) {
	exists, err := s.IdxRepo.Exists(indexName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("index not found")
	}
	docs, err := s.DocRepo.List(indexName)
	if err != nil {
		return nil, err
	}
	var results []map[string]interface{}
	for _, doc := range docs {
		if search == "" || contains(doc.Content, search) {
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(doc.Content), &m); err == nil {
				results = append(results, m)
			}
		}
	}
	return results, nil
}

// Content(JSON文字列)に部分一致するか判定（大文字小文字無視）
func contains(content string, search string) bool {
	if search == "" {
		return true
	}
	if search == "*" {
		return true
	}
	return strings.Contains(strings.ToLower(content), strings.ToLower(search))
}

func (s *DocumentService) GetDocument(ctx context.Context, indexName, key string) (map[string]interface{}, error) {
	exists, err := s.IdxRepo.Exists(indexName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("index not found")
	}
	doc, err := s.DocRepo.Find(indexName, key)
	if err != nil {
		if err == domain.ErrDocumentNotFound {
			return nil, fmt.Errorf("document not found")
		}
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
		return 0, fmt.Errorf("index not found")
	}
	return s.DocRepo.Count(indexName)
}
