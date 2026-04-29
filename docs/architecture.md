# Azure AI Search エミュレータ DDD風アーキテクチャ設計案

## ディレクトリ構成例

```
├── internal/
│   ├── api/                # Ginのルーティング・ハンドラ（インフラ層）
│   │   └── handler.go      # HTTPハンドラ群
│   ├── application/        # ユースケース層（サービス）
│   │   └── index_service.go
│   │   └── document_service.go
│   ├── domain/             # ドメイン層（エンティティ・リポジトリインターフェース）
│   │   └── index.go        # Indexエンティティ・リポジトリIF
│   │   └── document.go     # Documentエンティティ・リポジトリIF
│   └── infrastructure/     # インフラ層（DB実装など）
│       └── sqlite_index_repository.go
│       └── sqlite_document_repository.go
├── main.go             # エントリポイント、DI・サーバ起動
├── docs/               # ドキュメント
│   └── architecture.md # このファイル
```

## 各層の役割

- **internal/api/**
  - GinのルーティングとHTTPハンドラのみを記述。リクエスト/レスポンスのバリデーションやDTO変換もここ。
- **internal/application/**
  - ユースケース（サービス）層。ビジネスロジックのオーケストレーション。リポジトリIF経由で永続化層を呼び出す。
- **internal/domain/**
  - ドメインモデル（エンティティ、値オブジェクト）、リポジトリインターフェースを定義。ビジネスルールの中心。
- **internal/infrastructure/**
  - DBや外部サービスとのやりとり。リポジトリインターフェースの実装（例: SQLite）。
- **main.go**
  - DI（依存性注入）やサーバ起動、ルーティング初期化。

`internal/` 配下のパッケージはGoコンパイラにより外部モジュールからのインポートが禁止される。

## 例: Index作成の流れ

1. `internal/api/handler.go` でHTTPリクエストを受け、DTOを `internal/application/index_service.go` に渡す
2. `internal/application/index_service.go` でバリデーションや重複チェック等のユースケースロジックを実行
3. `internal/domain/index.go` のリポジトリIFを通じて `internal/infrastructure/sqlite_index_repository.go` でDB保存
4. 結果をDTOで返却

## メリット
- main.goが極小化され、責務が明確に分離
- テスト容易・拡張性高
- DDDの考え方に近い

---

ご要望に応じて、各層のサンプルコードや詳細設計もご提案可能です。
