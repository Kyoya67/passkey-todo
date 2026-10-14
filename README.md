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

## 考察

Go/Echo、PostgreSQL、sqlc、OpenAPI、AWS ECS Fargateを採用し、API仕様と実装のずれを減らしながら、ローカルとクラウドで同じAPIを確認できる構成にした。レイヤー分離、SQLの型安全なコード生成、TerraformによるAWS管理、GitHub Actionsによるデプロイを選んだ理由や設計上の判断は、[DESIGN.md](DESIGN.md)にまとめている。

## 作成したアプリケーションの起動・動作確認手順

### ローカル環境の起動

バックエンド、PostgreSQL、Swagger UIをDocker Composeで起動する。フロントエンドはNode.jsの静的HTTPサーバーで配信する。

### 前提

- Go
- Docker Desktop
- Docker Compose
- Node.js（`npx`を含む）
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

### フロントエンドの起動

別のターミナルでプロジェクトルートから実行する。

`````sh
cd frontend
npm ci
cd ..
npx serve frontend -l 3000
`````

ブラウザで`http://localhost:3000`を開く。ローカルホストでは`http://localhost:8080`、Amplifyのドメインでは`https://api.ca-techlounge-todo.kyo8.dev`をAPI URLとして自動設定する。API URLは画面から変更できず、「接続確認」ボタンでヘルスチェックだけを実行できる。

### マイグレーション

PostgreSQLコンテナが起動している状態で、プロジェクトルートから実行する。`golang-migrate`が適用済みバージョンを管理し、未適用のSQLだけを実行する。

`````sh
make db-migrate-up
`````

開発環境で直前のマイグレーションを一つ取り消す場合は、次を実行する。

`````sh
make db-migrate-down
`````

適用途中で失敗してdirty状態になった場合は、原因を修正した上で、対象バージョンを指定して状態を修正する。

`````sh
make db-migrate-force VERSION=1
`````

マイグレーションファイルは`backend/db/migration/001_create_todos.up.sql`と`001_create_todos.down.sql`のように、同じバージョン番号のup/downを追加する。

AWSのRDSへ適用する場合は、Terraformが作成するマイグレーション専用ECSタスクを実行する。`main`へのマイグレーションファイル変更では、CDがコミットハッシュをタグにしたイメージをECRへpushし、そのタグを指定したタスク定義を登録してからタスクを実行する。初回のイメージ登録や手動確認では、`make db-migrate-ecs`を利用できる。

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

### ローカル環境の動作確認

#### Swagger UIから確認する

ブラウザで`http://localhost:8081`を開く。各エンドポイントを選択して「Try it out」を押し、必要なパラメータやリクエストボディを入力して「Execute」を押す。レスポンスのステータスコードとJSONを確認する。

Swagger UIでは、次の4つの必須APIを順番に確認できる。

1. `POST /todos`でTODOを作成する
2. `GET /todos`で一覧を取得する
3. `PUT /todos/{id}`で作成したTODOを更新する
4. `DELETE /todos/{id}`で作成したTODOを削除する

最初に`GET /health`を実行し、HTTP 200と`{"status":"ok"}`が返ることを確認する。

#### curlから確認する

Swagger UIと同じ確認をコマンドラインで再現する場合は、次のコマンドを実行する。

`````sh
curl -i http://localhost:8080/health
`````

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

### ローカル環境の停止

プロジェクトルートでComposeのサービスを停止する。

`````sh
docker compose down
`````

`docker compose down -v`を実行するとPostgreSQLのボリュームも削除されるため、保存データを削除する場合以外は使用しない。

### クラウド環境の動作確認

AWS環境のAPIを確認するときは、プロジェクトルートでECS Serviceを起動し、状態を確認する。

`````sh
make ecs-start
make ecs-status
`````

`running`が`1`になったら、次のフロントエンドをブラウザで開く。

- [クラウドフロントエンド](https://ca-techlounge-todo.kyo8.dev)

画面の「接続確認」でAPIへ接続できることを確認する。画面からTODOを作成・一覧取得・タイトル検索・更新・削除し、必須APIの動作を確認する。

APIを直接確認する場合は、APIドメインを指定してOpenAPI定義にあるパスを実行する。API仕様の正は[api/openapi/openapi.yaml](api/openapi/openapi.yaml)であり、`paths`に具体的なHTTPメソッドとURL、`parameters`にクエリやパスパラメータ、`requestBody`にリクエスト形式、`responses`にレスポンスを定義している。

まずヘルスチェックを実行する。

`````sh
curl -i https://api.ca-techlounge-todo.kyo8.dev/health
`````

TODO APIは、OpenAPIの`/todos`と`/todos/{id}`に対応する。タイトルの部分一致検索は`title`クエリパラメータで実行する。

`````sh
curl -i -X POST https://api.ca-techlounge-todo.kyo8.dev/todos \
  -H 'Content-Type: application/json' \
  -d '{"title":"買い物をする"}'
curl -i 'https://api.ca-techlounge-todo.kyo8.dev/todos?title=買い物'
curl -i -X PUT https://api.ca-techlounge-todo.kyo8.dev/todos/1 \
  -H 'Content-Type: application/json' \
  -d '{"title":"買い物を完了する"}'
curl -i -X DELETE https://api.ca-techlounge-todo.kyo8.dev/todos/1
`````

確認後はECS Serviceを停止する。

`````sh
make ecs-stop
make ecs-status
`````

停止後もAWSリソース自体は削除されないため、利用しない時間帯は停止し、AWSコンソールまたはAWS CLIで状態と料金を確認する。
