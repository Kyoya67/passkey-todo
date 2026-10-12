# バックエンドテスト方針

Issue #5では、必須REST APIのHTTP処理、業務ルール、Repositoryのsqlc呼び出しを責務ごとにテストする。sqlcが生成したコード自体の単体テストは作成せず、生成コードを利用するRepositoryの引数、戻り値、エラー伝播をFakeで検証する。

## internal/handler

`internal/handler/todo_test.go`では、Fake Serviceを差し替え、HTTPリクエストのバインド、レスポンスJSON、ステータスコード、サービスエラーのHTTP変換、TODOのレスポンス変換をテストしている。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| todo.go | NewTodoHandler | 100.0% |
|  | GetHealth | 100.0% |
|  | CreateTodo | 100.0% |
|  | ListTodos | 88.2% |
|  | UpdateTodo | 90.9% |
|  | DeleteTodo | 100.0% |
|  | toTodo | 100.0% |
| **パッケージ合計** |  | **93.2%** |

## internal/service

`internal/service/todo_test.go`では、Fake Repositoryを差し替え、タイトルの入力検証、ページネーションのデフォルト値・上限値・不正値、Repositoryエラーの伝播、`pgx.ErrNoRows`から`ErrTodoNotFound`への変換をテストしている。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| todo.go | NewTodoService | 100.0% |
|  | Create | 100.0% |
|  | List | 100.0% |
|  | Update | 83.3% |
|  | Delete | 75.0% |
| **パッケージ合計** |  | **90.0%** |

## internal/repository

`internal/repository/todo_test.go`では、sqlcのクエリメソッドを持つFakeを差し替え、Create、List、Update、Deleteへ渡す引数、戻り値の受け渡し、Repositoryでのエラー伝播をテストしている。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| todo.go | NewTodoRepository | 100.0% |
|  | Create | 100.0% |
|  | List | 100.0% |
|  | Update | 100.0% |
|  | Delete | 100.0% |
| **パッケージ合計** |  | **100.0%** |

## カバレッジ計測結果

計測コマンド：`go test ./... -coverprofile=coverage.out`、`go tool cover -func=coverage.out`

| 対象 | カバレッジ |
|---|---:|
| internal/handler | 93.2% |
| internal/service | 90.0% |
| internal/repository | 100.0% |
| 全パッケージ | 29.3% |

全パッケージの値には、手動起動をエントリーポイントとする`cmd/api`、外部I/Oを扱うDB接続、sqlc・oapi-codegenの生成コード、Router、CORSミドルウェアが含まれる。生成コードは直接編集せず、外部I/Oを含む起動処理のテストは今後必要性を確認する。

## 検証コマンド

`backend`ディレクトリで次のコマンドを実行する。

- `go test ./...`
- `go test ./... -coverprofile=coverage.out`
- `go tool cover -func=coverage.out`
- `go vet ./...`
