package api

import (
	"ai-search-emulator/internal/application"
	"ai-search-emulator/internal/domain"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterHealthCheck(r *gin.Engine) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

func RegisterRoutes(r *gin.Engine, app *application.AppServices) {
	// インデックス作成API
	r.POST("/indexes", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		var req struct {
			Name       string      `json:"name" binding:"required"`
			Fields     interface{} `json:"fields" binding:"required"`
			Suggesters interface{} `json:"suggesters,omitempty"`
			Analyzers  interface{} `json:"analyzers,omitempty"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		err = app.IndexService.CreateIndex(c.Request.Context(), req.Name, io.NopCloser(bytes.NewReader(body)))
		if err != nil {
			if errors.Is(err, domain.ErrIndexAlreadyExists) {
				c.JSON(http.StatusConflict, gin.H{"error": "Index already exists"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.Data(http.StatusCreated, "application/json", body)
	})
	// ドキュメント追加API（1件追加、Azure仕様に準拠）
	r.POST("/indexes/:index/docs", func(c *gin.Context) {
		indexName := c.Param("index")
		var doc map[string]interface{}
		if err := c.ShouldBindJSON(&doc); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document body"})
			return
		}
		err := app.DocumentService.AddOrUpdateSingleDoc(c.Request.Context(), indexName, doc)
		if err != nil {
			if errors.Is(err, domain.ErrIndexNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else if errors.Is(err, domain.ErrMissingKeyField) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Document missing key field"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusCreated, doc)
	})
	// インデックス一覧取得API
	r.GET("/indexes", func(c *gin.Context) {
		selectFields := c.Query("$select")
		indexes, err := app.IndexService.ListIndexes(c.Request.Context(), selectFields)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"@odata.context": requestBaseURL(c.Request) + "/$metadata#indexes",
			"value":          indexes,
		})
	})
	// インデックス取得API
	r.GET("/indexes/:index", func(c *gin.Context) {
		indexName := c.Param("index")
		idx, err := app.IndexService.GetIndex(c.Request.Context(), indexName)
		if err != nil {
			if errors.Is(err, domain.ErrIndexNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusOK, idx)
	})
	// インデックス更新API（create-or-update）
	r.PUT("/indexes/:index", func(c *gin.Context) {
		indexName := c.Param("index")
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		var req struct {
			Name       string      `json:"name" binding:"required"`
			Fields     interface{} `json:"fields" binding:"required"`
			Suggesters interface{} `json:"suggesters,omitempty"`
			Analyzers  interface{} `json:"analyzers,omitempty"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		created, err := app.IndexService.CreateOrUpdateIndex(c.Request.Context(), indexName, io.NopCloser(bytes.NewReader(body)))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if created {
			c.Data(http.StatusCreated, "application/json", body)
		} else {
			c.Data(http.StatusOK, "application/json", body)
		}
	})
	// インデックス削除API
	r.DELETE("/indexes/:index", func(c *gin.Context) {
		indexName := c.Param("index")
		err := app.IndexService.DeleteIndex(c.Request.Context(), indexName)
		if err != nil {
			if errors.Is(err, domain.ErrIndexNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.Status(http.StatusNoContent)
	})
	// インデックス統計情報API
	r.GET("/indexes/:index/stats", func(c *gin.Context) {
		indexName := c.Param("index")
		stats, err := app.IndexService.GetIndexStats(c.Request.Context(), indexName)
		if err != nil {
			if errors.Is(err, domain.ErrIndexNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusOK, stats)
	})
	// ドキュメントバッチ操作API（upload/merge/mergeOrUpload/delete対応）
	r.POST("/indexes/:index/docs/index", func(c *gin.Context) {
		indexName := c.Param("index")
		var req struct {
			Value []map[string]interface{} `json:"value" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid batch request body"})
			return
		}
		results, err := app.DocumentService.BatchOperation(c.Request.Context(), indexName, req.Value)
		if err != nil {
			if errors.Is(err, domain.ErrIndexNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		statusCode := http.StatusOK
		for _, r := range results {
			if status, ok := r["status"].(bool); ok && !status {
				statusCode = http.StatusMultiStatus
				break
			}
		}
		c.JSON(statusCode, gin.H{"value": results})
	})
	// ドキュメント取得API
	r.GET("/indexes/:index/docs/:key", func(c *gin.Context) {
		indexName := c.Param("index")
		key := c.Param("key")
		doc, err := app.DocumentService.GetDocument(c.Request.Context(), indexName, key)
		if err != nil {
			if errors.Is(err, domain.ErrIndexNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else if errors.Is(err, domain.ErrDocumentNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusOK, doc)
	})
	// ドキュメント件数取得API
	r.GET("/indexes/:index/docs/$count", func(c *gin.Context) {
		indexName := c.Param("index")
		count, err := app.DocumentService.CountDocuments(c.Request.Context(), indexName)
		if err != nil {
			if errors.Is(err, domain.ErrIndexNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.String(http.StatusOK, "%d", count)
	})
	// ドキュメント検索API (GET)
	r.GET("/indexes/:index/docs", func(c *gin.Context) {
		indexName := c.Param("index")
		params := parseSearchParamsFromQuery(c)
		result, err := app.DocumentService.SearchDocuments(c.Request.Context(), indexName, params)
		if err != nil {
			handleSearchError(c, err)
			return
		}
		respondSearch(c, indexName, params, result)
	})
	// ドキュメント検索API (POST)
	r.POST("/indexes/:index/docs/search", func(c *gin.Context) {
		indexName := c.Param("index")
		params, err := parseSearchParamsFromBody(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		result, err := app.DocumentService.SearchDocuments(c.Request.Context(), indexName, params)
		if err != nil {
			handleSearchError(c, err)
			return
		}
		respondSearch(c, indexName, params, result)
	})
}

// parseSearchParamsFromQuery reads OData parameters from GET query string.
func parseSearchParamsFromQuery(c *gin.Context) application.SearchParams {
	top, _ := strconv.Atoi(c.Query("$top"))
	skip, _ := strconv.Atoi(c.Query("$skip"))
	count := strings.EqualFold(c.Query("$count"), "true")

	var selectFields []string
	if s := c.Query("$select"); s != "" {
		for _, f := range strings.Split(s, ",") {
			if f = strings.TrimSpace(f); f != "" {
				selectFields = append(selectFields, f)
			}
		}
	}

	var searchFields []string
	if s := c.Query("searchFields"); s != "" {
		for _, f := range strings.Split(s, ",") {
			if f = strings.TrimSpace(f); f != "" {
				searchFields = append(searchFields, f)
			}
		}
	}

	return application.SearchParams{
		Search:       c.Query("search"),
		Filter:       c.Query("$filter"),
		OrderBy:      c.Query("$orderby"),
		Select:       selectFields,
		SearchFields: searchFields,
		Top:          top,
		Skip:         skip,
		IncludeCount: count,
	}
}

// searchBody mirrors the Azure AI Search POST /docs/search request body.
type searchBody struct {
	Search       string `json:"search"`
	Filter       string `json:"$filter"`
	OrderBy      string `json:"$orderby"`
	Select       string `json:"$select"`
	SearchFields string `json:"searchFields"`
	Top          *int   `json:"$top"`
	Skip         *int   `json:"$skip"`
	Count        bool   `json:"$count"`
}

// parseSearchParamsFromBody reads OData parameters from a POST JSON body.
func parseSearchParamsFromBody(c *gin.Context) (application.SearchParams, error) {
	var b searchBody
	if err := c.ShouldBindJSON(&b); err != nil {
		return application.SearchParams{}, err
	}

	top := 0
	if b.Top != nil {
		top = *b.Top
	}
	skip := 0
	if b.Skip != nil {
		skip = *b.Skip
	}

	var selectFields []string
	if b.Select != "" {
		for _, f := range strings.Split(b.Select, ",") {
			if f = strings.TrimSpace(f); f != "" {
				selectFields = append(selectFields, f)
			}
		}
	}

	var searchFields []string
	if b.SearchFields != "" {
		for _, f := range strings.Split(b.SearchFields, ",") {
			if f = strings.TrimSpace(f); f != "" {
				searchFields = append(searchFields, f)
			}
		}
	}

	return application.SearchParams{
		Search:       b.Search,
		Filter:       b.Filter,
		OrderBy:      b.OrderBy,
		Select:       selectFields,
		SearchFields: searchFields,
		Top:          top,
		Skip:         skip,
		IncludeCount: b.Count,
	}, nil
}

func handleSearchError(c *gin.Context, err error) {
	if errors.Is(err, domain.ErrIndexNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
		return
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "invalid $filter") || strings.HasPrefix(msg, "invalid $orderby") {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return scheme + "://" + host
}

func buildNextLink(baseURL, indexName string, params application.SearchParams, total int64) string {
	top := params.Top
	if top <= 0 {
		top = application.DefaultTop
	}
	nextSkip := params.Skip + top
	if int64(nextSkip) >= total {
		return ""
	}
	u, _ := url.Parse(baseURL + "/indexes/" + url.PathEscape(indexName) + "/docs")
	q := url.Values{}
	if params.Search != "" {
		q.Set("search", params.Search)
	}
	if params.Filter != "" {
		q.Set("$filter", params.Filter)
	}
	if params.OrderBy != "" {
		q.Set("$orderby", params.OrderBy)
	}
	if len(params.Select) > 0 {
		q.Set("$select", strings.Join(params.Select, ","))
	}
	if len(params.SearchFields) > 0 {
		q.Set("searchFields", strings.Join(params.SearchFields, ","))
	}
	if params.Top > 0 {
		q.Set("$top", strconv.Itoa(params.Top))
	}
	q.Set("$skip", strconv.Itoa(nextSkip))
	if params.IncludeCount {
		q.Set("$count", "true")
	}
	// url.Values.Encode percent-encodes "$" as "%24"; Azure nextLink URLs
	// conventionally use the literal "$" which is a valid query char.
	u.RawQuery = strings.ReplaceAll(q.Encode(), "%24", "$")
	return u.String()
}

func respondSearch(c *gin.Context, indexName string, params application.SearchParams, result *application.SearchResult) {
	baseURL := requestBaseURL(c.Request)
	odataCtx := fmt.Sprintf("%s/indexes('%s')/$metadata#docs(*)", baseURL, indexName)

	docs := make([]map[string]interface{}, len(result.Value))
	for i, doc := range result.Value {
		d := make(map[string]interface{}, len(doc)+1)
		for k, v := range doc {
			d[k] = v
		}
		d["@search.score"] = 1.0
		docs[i] = d
	}

	resp := gin.H{
		"@odata.context":   odataCtx,
		"@search.coverage": 100.0,
		"value":            docs,
	}
	if params.IncludeCount {
		resp["@odata.count"] = result.Total
	}
	if nextLink := buildNextLink(baseURL, indexName, params, result.Total); nextLink != "" {
		resp["@odata.nextLink"] = nextLink
	}
	c.JSON(http.StatusOK, resp)
}

func ApiKeyAuthMiddleware() gin.HandlerFunc {
	apiKeyEnv := os.Getenv("API_KEY")
	if apiKeyEnv == "" {
		log.Println("[WARN] API_KEY is not set — all requests will be rejected")
	}
	return func(c *gin.Context) {
		apiKey := c.GetHeader("api-key")
		if apiKey == "" {
			apiKey = c.GetHeader("Api-Key")
		}
		if apiKeyEnv == "" || apiKey != apiKeyEnv {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key required or invalid"})
			return
		}
		c.Next()
	}
}
