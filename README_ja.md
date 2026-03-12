# Network Filter Proxy

Go で実装されたフォワードプロキシです。送信元 IP ベースのホワイトリストにより、HTTPS CONNECT および通常の HTTP リクエストをフィルタリングします。ルールはローカルの Management REST API を通じてランタイムに管理できます。

## 機能

- **HTTPS CONNECT トンネリング**と通常 HTTP フォワーディング（ポート 3128）
- **送信元 IP ベースのホワイトリスト**（デフォルト拒否: 未登録 IP は 403）
- **柔軟なマッチング**: 完全一致ドメイン、ワイルドカード (`*.example.com`)、IP アドレス、CIDR、ポート指定
- **Management REST API**（`127.0.0.1:8080` にバインド、localhost のみ）
- **ヘルスチェック**エンドポイント（稼働時間、アクティブ接続数、ルール数）
- **構造化ログ**（JSON/テキスト）`log/slog` 使用
- **Graceful Shutdown**（タイムアウト設定可能、CONNECT トンネルのクリーンアップ付き）
- **軽量 Docker イメージ**（< 30 MB、distroless ベース）

## クイックスタート

### バイナリ

```bash
go build -o filter-proxy ./cmd/filter-proxy
./filter-proxy
```

### Docker

```bash
docker build -t filter-proxy .
docker run -d --name filter-proxy -p 3128:3128 filter-proxy
```

> **注意:** Management API はコンテナ内で `127.0.0.1` にバインドされるため、ポートマッピングではホストから到達できません。API にアクセスするには `--network host` または `docker exec` を使用してください。

### ルール登録とテスト

バイナリ実行時は API を直接呼び出します:

```bash
# 10.0.0.5 から example.com への全ポートアクセスを許可
curl -X PUT http://127.0.0.1:8080/api/v1/rules/10.0.0.5 \
  -H 'Content-Type: application/json' \
  -d '{"entries":[{"host":"example.com"}]}'

# フィルタプロキシ経由でリクエスト
curl -x http://127.0.0.1:3128 http://example.com
```

Docker のデフォルトブリッジネットワークで実行している場合は `docker exec` を使用します:

```bash
docker exec filter-proxy wget -q -O- --method=PUT \
  --body-data='{"entries":[{"host":"example.com"}]}' \
  --header='Content-Type: application/json' \
  http://127.0.0.1:8080/api/v1/rules/10.0.0.5
```

## 設定

すべての設定は環境変数で制御します。

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `PROXY_PORT` | `3128` | プロキシのリッスンポート |
| `API_PORT` | `8080` | Management API ポート（常に `127.0.0.1` にバインド） |
| `LOG_FORMAT` | `json` | `json` または `text` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SHUTDOWN_TIMEOUT` | `30` | Graceful Shutdown のタイムアウト（秒） |

## API リファレンス

ベース URL: `http://127.0.0.1:8080`

OpenAPI 仕様書: [`docs/swagger/swagger.yaml`](docs/swagger/swagger.yaml)

### ヘルスチェック

```http
GET /api/v1/health
```

```json
{
  "status": "ok",
  "uptime_seconds": 3600,
  "active_connections": 5,
  "rule_count": 3
}
```

### 全ルール一覧

```http
GET /api/v1/rules
```

### 送信元 IP のルール設定

```http
PUT /api/v1/rules/{sourceIP}
```

```json
{
  "entries": [
    { "host": "api.example.com", "port": 443 },
    { "host": "*.github.com" },
    { "host": "10.0.0.0/8" }
  ]
}
```

- `host`（必須）: ドメイン、ワイルドカード (`*.example.com`)、IP、CIDR
- `port`（任意）: 許可するポート番号。`0` または省略で全ポート許可

### 送信元 IP のルール削除

```http
DELETE /api/v1/rules/{sourceIP}
```

### 全ルール削除

```http
DELETE /api/v1/rules
```

## アーキテクチャ

```plaintext
cmd/filter-proxy/       エントリポイント
internal/
  config/               環境変数の読み込み
  logger/               slog ファクトリ（JSON/text、timestamp フィールド）
  rule/                 インメモリルールストア（sync.RWMutex）+ マッチャー
  proxy/                プロキシハンドラ（goproxy ラッパー、トンネル追跡）
  api/                  Management REST API ハンドラ
```

### マッチングルール

| パターン | 例 | マッチ対象 |
|---------|-----|-----------|
| 完全一致ドメイン | `api.example.com` | `api.example.com` のみ |
| ワイルドカード | `*.example.com` | `example.com` と直下1階層（例: `api.example.com`） |
| IP アドレス | `140.82.112.3` | 完全一致（IPv4/IPv6 は `net.IP.Equal` で正規化） |
| CIDR | `10.0.0.0/8` | 範囲内の全 IP |

### セキュリティ

- **デフォルト拒否**: 未登録の送信元 IP からのリクエストは `403 Forbidden` で拒否
- **TLS パススルー**: プロキシは TLS を終端せず、CONNECT リクエストをエンドツーエンドでトンネル
- **localhost 限定 API**: Management API は `127.0.0.1` にのみバインド
- **入力バリデーション**: ホストパターン、ポート、送信元 IP、JSON ペイロードを厳密に検証

## 開発

### 前提条件

- Go 1.26 以上
- Node.js（textlint 用）

### テスト実行

```bash
# ユニットテスト + 結合テスト（E2E を除く）
go test -race $(go list ./... | grep -v '/e2e$')

# E2E テストのみ
go test -race ./e2e/...

# Lint
golangci-lint run ./...

# Markdown Lint
npm install
npm run textlint
```

### OpenAPI 仕様書の再生成

```bash
make swagger
```

### CI

CI パイプラインは **test**、**e2e**、**lint**、**openapi**（仕様書の最新性チェック）の4ジョブを並列実行し、続いて **build** ジョブで Docker イメージが 30 MB 以下であることを検証します。

### リリース

semver タグをプッシュするとリリースワークフローが起動します:

```bash
git tag v1.0.0
git push origin v1.0.0
```

linux/darwin (amd64/arm64) のバイナリビルド、GHCR への Docker イメージプッシュ、チェックサム付き GitHub Release の作成が自動で行われます。

## ライセンス

MIT
