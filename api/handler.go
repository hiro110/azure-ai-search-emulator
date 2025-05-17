package api

import (
	"ai-search-emulator/application"
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, app *application.AppServices) {
	// インデックス作成API
	r.POST("/indexes", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		var req struct {
			Name      string      `json:"name" binding:"required"`
			Fields    interface{} `json:"fields" binding:"required"`
			Suggesters interface{} `json:"suggesters,omitempty"`
			Analyzers  interface{} `json:"analyzers,omitempty"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		err = app.IndexService.CreateIndex(c.Request.Context(), req.Name, io.NopCloser(bytes.NewReader(body)))
		if err != nil {
			if err.Error() == "already exists" {
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
			if err.Error() == "index not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else if err.Error() == "missing key field" {
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
		// Azure仕様に合わせてvalue配列で返却
		c.JSON(http.StatusOK, gin.H{"value": indexes})
	})
	// インデックス取得API
	r.GET("/indexes/:index", func(c *gin.Context) {
		indexName := c.Param("index")
		idx, err := app.IndexService.GetIndex(c.Request.Context(), indexName)
		if err != nil {
			if err.Error() == "index not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusOK, idx)
	})
	// インデックス更新API
	r.PUT("/indexes/:index", func(c *gin.Context) {
		indexName := c.Param("index")
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		var req struct {
			Name      string      `json:"name" binding:"required"`
			Fields    interface{} `json:"fields" binding:"required"`
			Suggesters interface{} `json:"suggesters,omitempty"`
			Analyzers  interface{} `json:"analyzers,omitempty"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		err = app.IndexService.UpdateIndex(c.Request.Context(), indexName, io.NopCloser(bytes.NewReader(body)))
		if err != nil {
			if err.Error() == "index not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.Data(http.StatusOK, "application/json", body)
	})
	// インデックス削除API
	r.DELETE("/indexes/:index", func(c *gin.Context) {
		indexName := c.Param("index")
		err := app.IndexService.DeleteIndex(c.Request.Context(), indexName)
		if err != nil {
			if err.Error() == "index not found" {
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
			if err.Error() == "index not found" {
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
			if err.Error() == "index not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{"value": results})
	})
	// ドキュメント取得API
	r.GET("/indexes/:index/docs/:key", func(c *gin.Context) {
		indexName := c.Param("index")
		key := c.Param("key")
		doc, err := app.DocumentService.GetDocument(c.Request.Context(), indexName, key)
		if err != nil {
			if err.Error() == "index not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else if err.Error() == "document not found" {
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
			if err.Error() == "index not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.String(http.StatusOK, "%d", count)
	})
	// ドキュメント検索API
	r.GET("/indexes/:index/docs", func(c *gin.Context) {
		indexName := c.Param("index")
		search := c.Query("search")
		results, err := app.DocumentService.SearchDocuments(c.Request.Context(), indexName, search)
		if err != nil {
			if err.Error() == "index not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{"value": results})
	})
	r.POST("/indexes/:index/docs/search", func(c *gin.Context) {
		indexName := c.Param("index")
		var req struct {
			Search string `json:"search"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		results, err := app.DocumentService.SearchDocuments(c.Request.Context(), indexName, req.Search)
		if err != nil {
			if err.Error() == "index not found" {
				c.JSON(http.StatusNotFound, gin.H{"error": "Index not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{"value": results})
	})
}

func ApiKeyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("api-key")
		if apiKey == "" {
			apiKey = c.GetHeader("Api-Key")
		}
		if apiKey != "your-api-key" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key required or invalid"})
			return
		}
		c.Next()
	}
}
