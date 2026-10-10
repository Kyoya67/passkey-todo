# kyoya67-todo

## ローカル開発環境

現時点では、PostgreSQLをDockerコンテナで起動し、Go/Echoのバックエンドはホスト上で起動する。

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

### バックエンドの起動

別のターミナルで、`backend`ディレクトリから実行する。`godotenv`がプロジェクトルートの`.env`を読み込むため、手動で`export`する必要はない。

`````sh
cd backend
go run ./cmd/api
`````

起動時にPostgreSQLへ接続できない場合、バックエンドはエラー終了する。

### 動作確認

別のターミナルからHealth APIへリクエストを送信する。

`````sh
curl -i http://localhost:8080/health
`````

HTTP 200とJSONレスポンスが返れば、Echoサーバーが起動している。

### 停止

バックエンドを起動しているターミナルで`Ctrl+C`を押し、Goアプリを停止する。その後、プロジェクトルートでPostgreSQLを停止する。

`````sh
docker compose down
`````

`docker compose down -v`を実行するとPostgreSQLのボリュームも削除されるため、保存データを削除する場合以外は使用しない。

## 現時点の対象外

- Goアプリ用Dockerfile
- GoアプリとPostgreSQLのコンテナ起動
- TODO 4 APIの動作確認

これらはIssue #4以降で対応する。
