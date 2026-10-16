# バックエンドテスト方針

REST APIのHTTP処理、業務ルール、Repositoryのsqlc呼び出しを責務ごとにテストする。sqlcが生成したコード自体の単体テストは作成せず、生成コードを利用するRepositoryの引数、戻り値、エラー伝播をFakeで検証する。

## cmd/api

`main.go`では、CORS、TODOのOrigin検証、セッション検証、認証APIのレート制限を含む依存関係の組み立てを行う。`main`を直接起動する単体テストはなく、今回の計測では0.0%。判定処理は`internal/middleware`でテストする。

## internal/auth

`webauthn.go`では、次の観点をテストしている。ローカル・本番のRP IDとOriginでの初期化、不正な設定の拒否、discoverable credentialとユーザー検証の必須設定を確認する。

`handler.go`では、次の観点をテストしている。Fake ServiceによるHTTPステータスの変換、Origin・Content-Type・JSON・本文サイズの確認、内部エラーの非公開、登録開始レスポンスのキャッシュ抑止を確認する。

`service.go`では、次の観点をテストしている。保存先をFakeに差し替え、実WebAuthnライブラリと合成したnone attestationを使って登録成功、異なるChallenge・Origin・RP ID、ユーザー検証なし、期限切れ、ユーザーID不一致、再送を検証する。検証失敗時は保存先を呼ばず、登録IDを消費すること、DB障害と不正なCredentialを扱うことも確認する。

`repository.go`では、次の観点をテストしている。トランザクション開始失敗、実DBへのユーザーとCredentialの保存、Credential ID重複時にユーザー作成もロールバックすることを確認する。

`challenge.go`では、次の観点をテストしている。実RedisへのSessionData保存・復元、TTLによる期限切れ、登録IDの再使用拒否、8並列での消費成功が1回だけになること、破損データとRedis障害を確認する。HTTP→Service→実ライブラリ→Redis→実DBの登録成功・再送拒否も統合テストで確認する。端末の実パスキーを使うブラウザE2Eは未実施。

`handler.go`と`service.go`では、単一のHandler・Serviceによる登録・ログイン・ログアウトをテストする。discoverable credentialでのログイン開始、ログインIDの使い捨て、ユーザーハンド照合、合成署名を使った実WebAuthn検証、失敗時のCookie非発行、Origin制限、Cookie属性を確認する。ログアウトではOrigin拒否、セッションなし・期限切れ時の204、Cookie失効属性、Redis障害時の503を確認する。テストケースも`handler_test.go`と`service_test.go`にまとめている。`store.go`は`store_test.go`でFake Redisと実RedisによるTTL、キーの消費、セッション保存・読取・失効、無効・欠損・破損セッションの拒否を確認する。`repository.go`ではCredentialの検索・更新もテストする。ログイン全体の実DB統合テストは未実施。

ログイン失敗時はHandlerが入力解析とService検証の段階を固定ラベルで記録すること、Serviceが拒否理由を機密値なしで分類すること、クライアントやログに内部エラーとセッショントークンが出ないことを確認する。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| webauthn.go | NewWebAuthn | 100.0% |
| handler.go | NewHandler | 100.0% |
|  | BeginRegistration | 100.0% |
|  | FinishRegistration | 95.7% |
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
| handler.go | BeginLogin | 88.9% |
|  | FinishLogin | 82.9% |
|  | Logout | 100.0% |
| service.go | WithLogin | 100.0% |
|  | Error | 0.0% |
|  | Unwrap | 100.0% |
|  | rejectLogin | 100.0% |
|  | loginReason | 100.0% |
|  | Logout | 100.0% |
|  | BeginLogin | 91.7% |
|  | FinishLogin | 93.0% |
|  | WebAuthnID | 100.0% |
|  | WebAuthnName | 0.0% |
|  | WebAuthnDisplayName | 0.0% |
|  | WebAuthnCredentials | 100.0% |
| store.go | NewLoginStore | 100.0% |
|  | loginKey | 100.0% |
|  | Save | 83.3% |
|  | Consume | 77.8% |
|  | NewSessionStore | 100.0% |
|  | sessionKey | 100.0% |
|  | Save | 83.3% |
|  | Load | 92.9% |
|  | Revoke | 100.0% |
| repository.go | FindCredential | 90.9% |
|  | UpdateCredential | 77.8% |
| **パッケージ合計（統合テスト込み）** |  | **90.1%** |

今回の計測ではローカルのPostgreSQLとRedisを使用した。`WebAuthnCredentials`は新規ユーザーの空Credential一覧を返すインターフェース実装で、登録経路では呼ばれない。

## internal/middleware

`cors_test.go`では、許可OriginへのCookie付きリクエストのCORSヘッダーを確認する。`cors.go`の`CORS`は単体テストで100.0%。

`auth_test.go`では、TODO一覧・作成・更新・削除の未ログイン拒否、正しいセッションの通過、期限切れ・不正セッションの拒否、Redis障害時の503、公開ルートとCORSプリフライトの通過を確認する。ユーザーIDをリクエストコンテキストへ渡すことも確認する。TODOの所有者条件はTODOパッケージで検証する。

`csrf_test.go`では、TODOの状態変更で許可Originを通し、別Origin・Originなし・`null`・似たドメインを403にすることを確認する。GET・OPTIONS、認証・ヘルスルートはこのミドルウェアの対象外であることも確認する。

`auth_rate_limit_test.go`では、認証APIのルート別上限、超過時の429と`Retry-After`、Redis障害時の503、ALBの`X-Forwarded-For`から最終IPを採用すること、対象外ルートの通過を確認する。ローカルRedisを使う統合テストではLuaスクリプトによる10回までの通過・11回目の拒否とTTL設定を確認する。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| auth.go | UserID | 100.0% |
|  | WithUserID | 100.0% |
|  | RequireTodoSession | 100.0% |
| cors.go | CORS | 100.0% |
| csrf.go | RequireTodoOrigin | 100.0% |
| auth_rate_limit.go | AuthRateLimit | 100.0% |
|  | authRateLimit | 83.3% |
|  | albClientIP | 100.0% |
| **パッケージ合計** |  | **93.8%** |

## internal/config

`config.go`では、次の観点をテストしている。認証設定の未指定・両方指定・片方だけ指定、Redis接続先とCSRF許可Originの必須条件、DB等の必須設定、環境変数の既定値、CORS・CSRFのOrigin一覧の分割を確認する。RP ID・Origin・Redis接続先・CSRF許可Originのいずれかがない場合は起動設定エラーとする。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| config.go | Load | 100.0% |
|  | getenvDefault | 100.0% |
|  | splitCSV | 100.0% |
| **パッケージ合計** | | **100.0%** |

## internal/router

`router.go`では、次の観点をテストしている。渡された登録・ログイン・ログアウトHandlerとTODO Handlerが生成ルーティングへ接続され、Health APIへ到達すること、TODOルートが登録されていること、埋め込みOpenAPI定義が有効であることを確認する。RepositoryとServiceの組み立ては`cmd/api/main.go`が担当する。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| router.go | Register | 100.0% |
| **パッケージ合計** | | **100.0%** |

## internal/todo

`handler_test.go`では、Fake Serviceを差し替え、認証済みユーザーIDの引き渡しと未認証リクエストの401、HTTPリクエストのバインド、レスポンスJSON、ステータスコード、サービスエラーのHTTP変換、TODOのレスポンス変換をテストしている。503では操作名だけをログに残し、タイトルやDBエラー本文を漏らさないことも確認する。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| handler.go | NewHandler | 100.0% |
|  | serviceUnavailable | 100.0% |
|  | GetHealth | 100.0% |
|  | CreateTodo | 100.0% |
|  | ListTodos | 95.0% |
|  | UpdateTodo | 87.5% |
|  | MoveTodo | 84.6% |
|  | DeleteTodo | 100.0% |
|  | toTodo | 100.0% |
|  | requestPriorityLevel | 100.0% |
|  | requestStatus | 100.0% |
`service_test.go`では、Fake Repositoryを差し替え、認証済みユーザーIDの引き渡し、タイトルの入力検証、ページネーションのデフォルト値・上限値・不正値、Repositoryエラーの伝播、`pgx.ErrNoRows`から`ErrNotFound`への変換をテストしている。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| service.go | NewService | 100.0% |
|  | normalizeStatus | 100.0% |
|  | normalizePriorityLevel | 80.0% |
|  | Create | 100.0% |
|  | List | 100.0% |
|  | Update | 100.0% |
|  | Delete | 75.0% |
|  | Move | 83.3% |
`repository_test.go`では、sqlcのクエリメソッドを持つFakeを差し替え、Create、List、Update、Deleteへ渡す所有者IDを含む引数、戻り値の受け渡し、Repositoryでのエラー伝播をテストしている。`repository_integration_test.go`では、実DBでの所有者別一覧、他ユーザーによる更新・削除の拒否、`user_id`を指定しないINSERTの拒否などを検証する。以下は実DBを使った統合テスト込みの計測値。

| ファイル | 関数 | カバレッジ |
|---|---|---:|
| repository.go | NewRepository | 100.0% |
|  | Move | 81.2% |
|  | Create | 100.0% |
|  | List | 100.0% |
|  | Update | 100.0% |
|  | Delete | 100.0% |
| **パッケージ合計（統合テスト込み）** |  | **90.7%** |

## internal/todo（統合テスト）

`repository_integration_test.go`では、`INTEGRATION_DATABASE_URL`で指定したPostgreSQLへ接続し、Fakeを使わずにCreate、List、Update、Move、Deleteを実行している。所有者ごとの分離、`user_id`なしのINSERT拒否、作成したTODOの保存、一覧取得、ステータスの保存・更新・省略時の維持、不正なステータスのDB制約、優先順位の初期値・同じ状態と優先度のグループ内移動・異なる優先度、所有者、ステータスへの移動拒否、優先度変更後の順位、完了TODOの更新日時順と並べ替え拒否、更新後のタイトルと`updated_at`、削除後の更新失敗を検証する。

今回、ローカルPostgreSQLを使う`TestRepositoryIntegration`が成功した。統合テスト単独での`repository.go`のカバレッジは、`Move`が81.2%、`Create`・`List`・`Update`・`Delete`がそれぞれ100.0%。TODOパッケージ全体では24.6%。

## カバレッジ計測結果

ローカルPostgreSQLとRedisの接続先を指定して`go test ./... -coverprofile=coverage.out -count=1`を実行し、`go tool cover -func=coverage.out`で確認した結果。DB・Redis統合テストも実行済み。全パッケージの数値には、直接テストしていない起動処理と生成コードも含む。

| 対象パッケージ | カバレッジ |
|---|---:|
| cmd/api | 0.0% |
| internal/auth | 90.1% |
| internal/config | 100.0% |
| internal/database | 22.2% |
| internal/database/sqlc | 0.0% |
| internal/middleware | 93.8% |
| internal/openapi | 0.0% |
| internal/router | 100.0% |
| internal/todo | 90.7% |
| **全パッケージ合計** | **65.9%** |

統合テストだけを`go test ./internal/auth ./internal/middleware ./internal/todo -run Integration -coverprofile=integration-only-coverage.out -count=1 -v`で再実行し、authの4件、middlewareの1件、TODOの1件がすべて成功した。統合テスト単独のパッケージカバレッジはauth 45.4%、middleware 35.4%、TODO 24.6%。これは統合テストだけの値であり、上表の全テスト込みの値とは異なる。

全パッケージの値には、手動起動をエントリーポイントとする`cmd/api`、DB接続、sqlc・oapi-codegenの生成コード、Router、CORSミドルウェアが含まれる。生成コードは直接編集していない。Goのパッケージ単位の計測では、テスト対象パッケージ外にある生成sqlcコードは0.0%と表示されるため、Repository統合テストの実行有無とは区別する。

## 検証コマンド

`backend`ディレクトリで次のコマンドを実行する。

- `go test ./...`
- `go test ./... -coverprofile=coverage.out`
- `go tool cover -func=coverage.out`
- `go vet ./...`

PostgreSQL・Redisの統合テストは、DBコンテナ、マイグレーション、Redisを準備し、両方の接続先を指定して実行する。

- `INTEGRATION_DATABASE_URL='postgres://todo:secret@localhost:5432/todo?sslmode=disable' REDIS_TEST_ADDR='127.0.0.1:6379' go test ./... -coverprofile=coverage.out -count=1`
