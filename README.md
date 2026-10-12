# kyoya67-todo

## ドキュメント全体像

| ドキュメント | 内容 |
|---|---|
| [ASSIGNMENT.md](ASSIGNMENT.md) | 選考課題の要件、提出条件、注意事項 |
| [DESIGN.md](DESIGN.md) | 開発方針、構成、技術選定、設計判断 |
| [AGENTS.md](AGENTS.md) | 開発時の実装・ドキュメント・AI利用記録のルール |
| [backend/TESTING.md](backend/TESTING.md) | 単体テスト、統合テスト、カバレッジ、検証コマンド |
| [api/openapi/openapi.yaml](api/openapi/openapi.yaml) | REST API仕様の正となるOpenAPI定義 |
| [ai-usage/](ai-usage/) | PRごとの生成AI利用記録 |

実装を始める前に`ASSIGNMENT.md`と`DESIGN.md`を確認する。ローカル起動はこのREADME、テストの詳細は[backend/TESTING.md](backend/TESTING.md)を参照する。

## ローカル開発環境

バックエンド、PostgreSQL、Swagger UIをDocker Composeで起動する。

### 前提

- Go
- Docker Desktop
- Docker Compose
- sqlc

### 環境変数

プロジェクト直下の`.env.example`をコピーして`.env`を作成する。

`````sh
cp .env.example .env
`````

### コンテナの起動

プロジェクトルートで実行する。

`````sh
docker compose config
docker compose up -d --build
docker compose ps
`````

`backend`、`db`、`swagger-ui`が起動し、`db`サービスが`healthy`になっていることを確認する。Swagger UIは`http://localhost:8081`で利用できる。

### マイグレーション

PostgreSQLコンテナが起動している状態で、プロジェクトルートから実行する。`backend/db/migration/up`配下のSQLをファイル名順にすべて適用する。

`````sh
make db-migrate-up
`````

取り消しが必要な開発環境では、`backend/db/migration/down`配下のSQLを新しいものから逆順に実行する。

`````sh
make db-migrate-down
`````

マイグレーションファイルを追加した場合は、`up`と`down`の両方を追加する。現時点では適用済みマイグレーションの管理機能は導入していないため、適用済みのSQLを再実行すると失敗する場合がある。

テーブルを確認する。

`````sh
docker compose exec db \
  psql -U todo -d todo -c "\\dt"
`````

### sqlcのコード生成・検証

`sqlc.yaml`がある`backend`ディレクトリで実行する。

`````sh
cd backend
sqlc vet
sqlc generate
go fmt ./...
go test ./...
`````

### 動作確認

別のターミナルからHealth APIへリクエストを送信する。

`````sh
curl -i http://localhost:8080/health
`````

HTTP 200とJSONレスポンスが返れば、Echoサーバーが起動している。

TODO APIの確認例：

`````sh
curl -i -X POST http://localhost:8080/todos \
  -H 'Content-Type: application/json' \
  -d '{"title":"買い物をする"}'
curl -i 'http://localhost:8080/todos?title=買い物'
curl -i -X PUT http://localhost:8080/todos/1 \
  -H 'Content-Type: application/json' \
  -d '{"title":"買い物を完了する"}'
curl -i -X DELETE http://localhost:8080/todos/1
`````

### テストとカバレッジ

詳細は[backend/TESTING.md](backend/TESTING.md)を参照する。`backend`ディレクトリで実行する。

`````sh
cd backend
go test ./...
go vet ./...
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
`````

PostgreSQLコンテナとマイグレーションを準備した状態で、統合テストを実行する。

`````sh
INTEGRATION_DATABASE_URL='postgres://todo:secret@localhost:5432/todo?sslmode=disable' \
  go test ./internal/repository -run Integration -coverprofile=integration-only-coverage.out -count=1
go tool cover -func=integration-only-coverage.out
`````

単体テストのカバレッジはHandler 93.2%、Service 90.0%、Repository 100.0%、全パッケージ29.3%。統合テスト単独のRepositoryカバレッジは100.0%である。カバレッジプロファイルは生成物のためGitへコミットしない。

### OpenAPIとコード生成

OpenAPI定義は[api/openapi/openapi.yaml](api/openapi/openapi.yaml)で管理する。Swagger UIはCompose起動後に`http://localhost:8081`で確認できる。

Goコードを再生成する場合は`backend`ディレクトリで実行する。

`````sh
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 \
  -config internal/openapi/oapi-codegen.yaml \
  ../api/openapi/openapi.yaml
`````

生成コードは直接編集せず、OpenAPI定義を修正して再生成する。

### 生成AIの利用

開発中にOpenAI Codexを利用している。利用箇所、採用理由、開発者による修正・判断、検証方法、代表的なプロンプトは[ai-usage/](ai-usage/)にPRごとに記録している。

### 提出前の確認

- 選考課題の提出期限は受験開始2週間後の23:59である
- リポジトリは`catechlounge` Organization内のPrivateリポジトリとして管理する
- [ASSIGNMENT.md](ASSIGNMENT.md)の必須要件と提出条件を確認する
- ローカル起動、4つの必須API、テスト、OpenAPI仕様を確認する

### 停止

プロジェクトルートでComposeのサービスを停止する。

`````sh
docker compose down
`````

`docker compose down -v`を実行するとPostgreSQLのボリュームも削除されるため、保存データを削除する場合以外は使用しない。
