package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"ai-search-emulator/internal/domain"
)

type IndexService struct {
	Repo    domain.IndexRepository
	DocRepo domain.DocumentRepository
}

func NewIndexService(repo domain.IndexRepository, docRepo domain.DocumentRepository) *IndexService {
	return &IndexService{Repo: repo, DocRepo: docRepo}
}

func (s *IndexService) CreateIndex(ctx context.Context, name string, body io.ReadCloser) error {
	defer body.Close()

	exists, err := s.Repo.Exists(name)
	if err != nil {
		return err
	}
	if exists {
		return domain.ErrIndexAlreadyExists
	}

	// bodyからスキーマJSONを読み込む
	schemaBytes, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	// バリデーション: fields配列が存在するか最低限チェック
	var tmp struct {
		Fields []interface{} `json:"fields"`
	}
	if err := json.Unmarshal(schemaBytes, &tmp); err != nil {
		return fmt.Errorf("invalid schema json: %w", err)
	}
	if len(tmp.Fields) == 0 {
		return fmt.Errorf("fields required in schema")
	}

	// domain.Indexエンティティを生成し保存
	index := &domain.Index{
		Name:   name,
		Schema: string(schemaBytes),
	}
	if err := s.Repo.Create(index); err != nil {
		return err
	}
	return nil
}

func (s *IndexService) ListIndexes(ctx context.Context, selectFields string) ([]map[string]interface{}, error) {
	list, err := s.Repo.List()
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	for _, idx := range list {
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(idx.Schema), &schema); err != nil {
			continue // スキーマ不正はスキップ
		}
		if selectFields != "" && selectFields != "*" {
			fields := map[string]struct{}{}
			for _, f := range strings.Split(selectFields, ",") {
				fields[strings.TrimSpace(f)] = struct{}{}
			}
			filtered := map[string]interface{}{}
			for k, v := range schema {
				if _, ok := fields[k]; ok {
					filtered[k] = v
				}
			}
			result = append(result, filtered)
		} else {
			result = append(result, schema)
		}
	}
	return result, nil
}

func (s *IndexService) GetIndex(ctx context.Context, name string) (map[string]interface{}, error) {
	idx, err := s.Repo.FindByName(name)
	if err != nil {
		return nil, err
	}
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(idx.Schema), &schema); err != nil {
		return nil, fmt.Errorf("schema parse error")
	}
	return schema, nil
}

// CreateOrUpdateIndex upserts an index: creates it if absent, updates it if present.
// Returns true if the index was newly created, false if it was updated.
func (s *IndexService) CreateOrUpdateIndex(ctx context.Context, name string, body io.ReadCloser) (bool, error) {
	defer body.Close()
	schemaBytes, err := io.ReadAll(body)
	if err != nil {
		return false, fmt.Errorf("failed to read body: %w", err)
	}
	var tmp struct {
		Fields []interface{} `json:"fields"`
	}
	if err := json.Unmarshal(schemaBytes, &tmp); err != nil {
		return false, fmt.Errorf("invalid schema json: %w", err)
	}
	if len(tmp.Fields) == 0 {
		return false, fmt.Errorf("fields required in schema")
	}

	exists, err := s.Repo.Exists(name)
	if err != nil {
		return false, err
	}
	if exists {
		idx, err := s.Repo.FindByName(name)
		if err != nil {
			return false, err
		}
		idx.Schema = string(schemaBytes)
		return false, s.Repo.Update(idx)
	}
	return true, s.Repo.Create(&domain.Index{Name: name, Schema: string(schemaBytes)})
}

func (s *IndexService) UpdateIndex(ctx context.Context, name string, body io.ReadCloser) error {
	defer body.Close()
	idx, err := s.Repo.FindByName(name)
	if err != nil {
		return err
	}
	schemaBytes, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	// バリデーション: fields配列が存在するか最低限チェック
	var tmp struct {
		Fields []interface{} `json:"fields"`
	}
	if err := json.Unmarshal(schemaBytes, &tmp); err != nil {
		return fmt.Errorf("invalid schema json: %w", err)
	}
	if len(tmp.Fields) == 0 {
		return fmt.Errorf("fields required in schema")
	}
	idx.Schema = string(schemaBytes)
	return s.Repo.Update(idx)
}

func (s *IndexService) DeleteIndex(ctx context.Context, name string) error {
	return s.Repo.Delete(name)
}

func (s *IndexService) GetIndexStats(ctx context.Context, name string) (map[string]interface{}, error) {
	_, err := s.Repo.FindByName(name)
	if err != nil {
		return nil, err
	}
	// ドキュメント件数取得
	count, err := s.DocRepo.Count(name)
	if err != nil {
		return nil, err
	}
	// Azure仕様に合わせたレスポンス例
	return map[string]interface{}{
		"documentCount": count,
		"storageSize":  0, // SQLiteでは簡易的に0固定
	}, nil
}
