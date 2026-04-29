package main

import (
	"crypto/tls"
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"

	"ai-search-emulator/internal/api"
	"ai-search-emulator/internal/application"
	"ai-search-emulator/internal/infrastructure"
)

func dbPath() string {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p
	}
	return "./data.db"
}

func setupDB() *sql.DB {
	db, err := sql.Open("sqlite3", dbPath())
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
	_ = godotenv.Load()

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
	api.RegisterHealthCheck(r)
	r.Use(api.ApiKeyAuthMiddleware())
	api.RegisterRoutes(r, appServices)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	handler := api.ODataPathRewriter(r)

	// Start optional TLS listener for SDKs that require HTTPS (e.g. Azure.Search.Documents for C#).
	// Uses a self-signed certificate generated at startup — for local development only.
	if tlsPort := os.Getenv("TLS_PORT"); tlsPort != "" {
		cert, err := api.GenerateSelfSignedCert()
		if err != nil {
			log.Fatal("failed to generate TLS cert: ", err)
		}
		tlsServer := &http.Server{
			Addr:      ":" + tlsPort,
			Handler:   handler,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		}
		go func() {
			log.Printf("TLS server listening on :%s (self-signed cert)\n", tlsPort)
			if err := tlsServer.ListenAndServeTLS("", ""); err != nil {
				log.Fatal(err)
			}
		}()
	}

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
