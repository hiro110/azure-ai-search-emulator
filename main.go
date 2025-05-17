package main

import (
	"database/sql"
	"log"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"ai-search-emulator/api"
	"ai-search-emulator/application"
	"ai-search-emulator/infrastructure"
)

func setupDB() *sql.DB {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		log.Fatal(err)
	}
	// テーブル自動作成
	_, err = db.Exec(`
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
	);
	`)
	if err != nil {
		log.Fatal("failed to create tables: ", err)
	}
	return db
}

func main() {
	db := setupDB()
	defer db.Close()

	// リポジトリ実装
	indexRepo := infrastructure.NewSQLiteIndexRepository(db)
	docRepo := infrastructure.NewSQLiteDocumentRepository(db)

	// サービス層
	appServices := &application.AppServices{
		IndexService:    application.NewIndexService(indexRepo, docRepo),
		DocumentService: application.NewDocumentService(docRepo, indexRepo),
	}

	r := gin.Default()
	r.Use(api.ApiKeyAuthMiddleware())
	api.RegisterRoutes(r, appServices)

	r.Run(":8080")
}
