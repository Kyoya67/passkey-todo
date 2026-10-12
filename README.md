# kyoya67-todo

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

### PostgreSQLの起動

プロジェクトルートで実行する。

`````sh
docker compose config
docker compose up -d
docker compose ps
`````

`db`サービスが`healthy`になっていることを確認する。

### マイグレーション

PostgreSQLコンテナが起動している状態で、プロジェクトルートから実行する。

`````sh
docker compose exec -T db \
  psql -U todo -d todo \
  < backend/db/migration/001_create_todos.sql
`````

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

### アプリケーションの起動

プロジェクトルートで実行する。ComposeがPostgreSQLのhealthcheck成功後にバックエンドを起動する。

`````sh
docker compose up -d --build
docker compose ps
`````

`backend`と`db`が起動し、`db`が`healthy`になっていることを確認する。Swagger UIは`http://localhost:8081`で利用できる。

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

### 停止

プロジェクトルートでComposeのサービスを停止する。

`````sh
docker compose down
`````

`docker compose down -v`を実行するとPostgreSQLのボリュームも削除されるため、保存データを削除する場合以外は使用しない。
