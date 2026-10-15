# バックエンドテスト方針

Issue #5では、必須REST APIのHTTP処理、業務ルール、Repositoryのsqlc呼び出しを責務ごとにテストする。sqlcが生成したコード自体の単体テストは作成せず、生成コードを利用するRepositoryの引数、戻り値、エラー伝播をFakeで検証する。

## internal/auth

`webauthn.go`では、次の観点をテストしている。ローカル・本番のRP IDとOriginでの初期化、不正な設定の拒否、discoverable credentialとユーザー検証の必須設定を確認する。

`handler.go`では、次の観点をテストしている。Fake ServiceによるHTTPステータスの変換、Origin・Content-Type・JSON・本文サイズの確認、内部エラーの非公開、登録開始レスポンスのキャッシュ抑止を確認する。

`service.go`では、次の観点をテストしている。保存先をFakeに差し替え、実WebAuthnライブラリと合成したnone attestationを使って登録成功、異なるChallenge・Origin・RP ID、ユーザー検証なし、期限切れ、ユーザーID不一致、再送を検証する。検証失敗時は保存先を呼ばず、登録IDを消費すること、DB障害と不正なCredentialを扱うことも確認する。

`repository.go`では、次の観点をテストしている。トランザクション開始失敗、実DBへのユーザーとCredentialの保存、Credential ID重複時にユーザー作成もロールバックすることを確認する。

`challenge.go`では、次の観点をテストしている。実RedisへのSessionData保存・復元、TTLによる期限切れ、登録IDの再使用拒否、8並列での消費成功が1回だけになること、破損データとRedis障害を確認する。HTTP→Service→実ライブラリ→Redis→実DBの登録成功・再送拒否も統合テストで確認する。端末の実パスキーを使うブラウザE2Eは未実施。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| webauthn.go | NewWebAuthn | 100.0% |
| handler.go | NewHandler | 100.0% |
|  | BeginRegistration | 100.0% |
|  | FinishRegistration | 100.0% |
|  | checkOrigin | 100.0% |
| service.go | NewService | 100.0% |
|  | BeginRegistration | 82.4% |
|  | FinishRegistration | 100.0% |
|  | WebAuthnID | 100.0% |
|  | WebAuthnName | 100.0% |
|  | WebAuthnDisplayName | 100.0% |
|  | WebAuthnCredentials | 0.0% |
| repository.go | NewRepository | 100.0% |
|  | CreateAccount | 81.2% |
| challenge.go | NewChallengeStore | 100.0% |
|  | registrationKey | 100.0% |
|  | Save | 91.7% |
|  | Consume | 100.0% |
| **パッケージ合計（統合テスト含む）** |  | **93.2%** |

計測は`INTEGRATION_DATABASE_URL`に専用テストDB、`REDIS_TEST_ADDR`にローカルRedisを指定して`go test ./... -coverprofile=/tmp/kyoya-auth-redis-coverage.out -count=1`、`go tool cover -func=/tmp/kyoya-auth-redis-coverage.out`で行った。PostgreSQLには001〜002が必要。`go test -race ./internal/auth -count=1`も同じ設定で成功した。実行時だけ作成したテストDBは終了後に削除し、開発DBは変更していない。Redisの検証にはランダムなキーだけを使い、他のキーは削除していない。`WebAuthnCredentials`は新規ユーザーの空Credential一覧を返すインターフェース実装で、現在の登録経路では呼ばれない。

## internal/config

`config.go`では、次の観点をテストしている。認証設定の未指定・両方指定・片方だけ指定、Redis接続先の必須条件、DB等の必須設定、環境変数の既定値、Origin一覧の分割を確認する。RP ID・Origin・Redis接続先のいずれかがない場合は起動設定エラーとする。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| config.go | Load | 100.0% |
|  | getenvDefault | 100.0% |
|  | splitCSV | 100.0% |
| **パッケージ合計** | | **100.0%** |

## internal/router

`router.go`では、次の観点をテストしている。渡された登録HandlerとTODO Handlerが生成ルーティングへ接続され、Health APIへ到達すること、TODOルートが登録されていること、埋め込みOpenAPI定義が有効であることを確認する。RepositoryとServiceの組み立ては`cmd/api/main.go`が担当する。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| router.go | Register | 100.0% |
| **パッケージ合計** | | **100.0%** |

## internal/todo

`handler_test.go`では、Fake Serviceを差し替え、HTTPリクエストのバインド、レスポンスJSON、ステータスコード、サービスエラーのHTTP変換、TODOのレスポンス変換をテストしている。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| handler.go | NewHandler | 100.0% |
|  | GetHealth | 100.0% |
|  | CreateTodo | 100.0% |
|  | ListTodos | 88.2% |
|  | UpdateTodo | 90.9% |
|  | DeleteTodo | 100.0% |
|  | toTodo | 100.0% |
`service_test.go`では、Fake Repositoryを差し替え、タイトルの入力検証、ページネーションのデフォルト値・上限値・不正値、Repositoryエラーの伝播、`pgx.ErrNoRows`から`ErrNotFound`への変換をテストしている。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| service.go | NewService | 100.0% |
|  | Create | 100.0% |
|  | List | 100.0% |
|  | Update | 83.3% |
|  | Delete | 75.0% |
`repository_test.go`では、sqlcのクエリメソッドを持つFakeを差し替え、Create、List、Update、Deleteへ渡す引数、戻り値の受け渡し、Repositoryでのエラー伝播をテストしている。`repository_integration_test.go`では、PostgreSQLへ接続し、TODOの実保存、一覧取得、更新、更新日時、削除を検証している。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| repository.go | NewRepository | 100.0% |
|  | Create | 100.0% |
|  | List | 100.0% |
|  | Update | 100.0% |
|  | Delete | 100.0% |
| **パッケージ合計（単体テスト）** |  | **92.9%** |

## internal/todo（統合テスト）

`repository_integration_test.go`では、`INTEGRATION_DATABASE_URL`で指定したPostgreSQLへ接続し、Fakeを使わずにCreate、List、Update、Deleteを実行している。作成したTODOの保存、一覧取得、更新後のタイトル、`updated_at`の更新、削除後の更新失敗を検証している。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| repository.go | NewRepository | 100.0% |
|  | Create | 100.0% |
|  | List | 100.0% |
|  | Update | 100.0% |
|  | Delete | 100.0% |
| **Repository関数合計** |  | **100.0%** |

## カバレッジ計測結果

以下の表はIssue #5時点の旧パッケージ構成で計測した履歴。現行の単体テスト計測（`go test ./... -cover`）では、TODO 92.9%、auth 68.6%、config 100.0%、router 100.0%、database 22.2%だった。外部DB・Redisを使う統合テストは環境変数未指定時にスキップされるため、これは統合テスト込みの値ではない。

単体テストの計測コマンド：`go test ./... -coverprofile=coverage.out`、`go tool cover -func=coverage.out`

| 対象 | カバレッジ |
|---|---:|
| internal/handler | 93.2% |
| internal/service | 90.0% |
| internal/repository | 100.0% |
| 全パッケージ | 29.3% |

統合テスト単独の計測コマンド：`INTEGRATION_DATABASE_URL='postgres://todo:secret@localhost:5432/todo?sslmode=disable' go test ./internal/todo -run Integration -coverprofile=integration-only-coverage.out -count=1`、`go tool cover -func=integration-only-coverage.out`

| 統合テスト対象 | カバレッジ |
|---|---:|
| Repository関数（Issue #5時点） | 100.0% |
| 統合テスト全体（Issue #5時点） | 100.0% |

認証チャレンジをRedisへ移した時点の旧パッケージ構成での計測値は、auth 93.2%、config 100.0%、router 100.0%、既存TODOのHandler 93.2%、Service 90.0%、Repository 100.0%、全パッケージ50.0%。Go文単位の割合と、実DBやブラウザまで含む検証範囲は別の指標として扱う。

全パッケージの値には、手動起動をエントリーポイントとする`cmd/api`、外部I/Oを扱うDB接続、sqlc・oapi-codegenの生成コード、Router、CORSミドルウェアが含まれる。生成コードは直接編集していない。Repositoryの統合テストは、PostgreSQL接続URLを指定した場合に実行する。

## 検証コマンド

`backend`ディレクトリで次のコマンドを実行する。

- `go test ./...`
- `go test ./... -coverprofile=coverage.out`
- `go tool cover -func=coverage.out`
- `go vet ./...`

PostgreSQLの統合テストは、DBコンテナとマイグレーションを準備した上で実行する。

- `INTEGRATION_DATABASE_URL='postgres://todo:secret@localhost:5432/todo?sslmode=disable' go test ./internal/todo -run Integration -count=1`
