# 設計

> このドキュメントは AIエージェント（Claude Code 等）が実装を行うことを前提としています。
> 不明な情報が発生した場合は、実装前に必ず確認を取ってください。

## 情報の明確性チェック

### ユーザーから明示された情報

- [x] 言語: Go 1.26
- [x] フォワードプロキシライブラリ: `elazarl/goproxy`
- [x] ポート: `:3128`（プロキシ）、`:8080`（Management API）
- [x] ルール保持: メモリ（`sync.RWMutex` + `map`）、永続化なし
- [x] ログ: `log/slog`（JSON / テキスト）
- [x] Graceful Shutdown: `net/http` の `Server.Shutdown()`
- [x] Docker ベースイメージ: `gcr.io/distroless/static`（static binary）
- [x] 品質基準: `go test -race` / `golangci-lint` / カバレッジ 80%

### 不明/要確認の情報

なし（全要件が明示済み）

---

## アーキテクチャ概要

```mermaid
graph TD
    subgraph Docker internal network
        CA[Claude Container A<br/>HTTP_PROXY=proxy:3128]
        CB[Claude Container B<br/>HTTP_PROXY=proxy:3128]
        FP[Filter Proxy Container<br/>:3128 proxy / :8080 API]
        CA -->|CONNECT| FP
        CB -->|CONNECT| FP
    end

    subgraph Filter Proxy
        PH[ProxyHandler<br/>elazarl/goproxy]
        AH[APIHandler<br/>net/http]
        RS[RuleStore<br/>sync.RWMutex + map]
        M[Matcher<br/>domain/wildcard/CIDR]
        L[Logger<br/>log/slog]
        PH --> RS
        PH --> M
        PH --> L
        AH --> RS
        AH --> L
    end

    FP -->|ホワイトリスト通過| Internet
    FP -->|403 Forbidden| CA
    ClaudeWork -->|PUT /rules/{IP}| AH
```

---

## パッケージ構成

```text
filter-proxy/
├── cmd/
│   └── filter-proxy/
│       └── main.go           # エントリポイント・DI・シグナル処理
├── internal/
│   ├── config/
│   │   └── config.go         # 環境変数設定
│   ├── rule/
│   │   ├── store.go          # RuleStore（CRUD + sync.RWMutex）
│   │   ├── matcher.go        # Matches / ValidateEntry
│   │   ├── store_test.go
│   │   └── matcher_test.go
│   ├── proxy/
│   │   ├── handler.go        # ProxyHandler（goproxy ラッパー）
│   │   └── handler_test.go
│   ├── api/
│   │   ├── handler.go        # APIHandler（REST API）
│   │   └── handler_test.go
│   └── logger/
│       └── logger.go         # slog ファクトリ
├── Dockerfile
├── go.mod
└── go.sum
```

---

## コンポーネント一覧

| コンポーネント名 | 目的 | 詳細リンク |
|-----------------|------|-----------|
| RuleStore | ルールセットのメモリ内 CRUD・スレッドセーフ管理 | [詳細](components/rule-store.md) @components/rule-store.md |
| Matcher | ドメイン・ワイルドカード・IP・CIDR マッチング | [詳細](components/matcher.md) @components/matcher.md |
| ProxyHandler | HTTP CONNECT フィルタリング・TLS パススルー | [詳細](components/proxy-handler.md) @components/proxy-handler.md |
| APIHandler | Management REST API（ルール CRUD・ヘルスチェック） | [詳細](components/api-handler.md) @components/api-handler.md |
| Logger | 構造化ログ（JSON / テキスト）出力 | [詳細](components/logger.md) @components/logger.md |
| Server（main） | 起動・DI・Graceful Shutdown | [詳細](components/server.md) @components/server.md |

---

## API 一覧

| メソッド | パス | 目的 | 詳細リンク |
|---------|------|------|-----------|
| GET | `/api/v1/health` | ヘルスチェック | [詳細](api/management.md#get-apiv1health) @api/management.md |
| GET | `/api/v1/rules` | 全ルール一覧取得 | [詳細](api/management.md#get-apiv1rules) @api/management.md |
| PUT | `/api/v1/rules/{sourceIP}` | ルールセット全置換 | [詳細](api/management.md#put-apiv1rulessourceip) @api/management.md |
| DELETE | `/api/v1/rules/{sourceIP}` | 指定 IP のルール削除 | [詳細](api/management.md#delete-apiv1rulessourceip) @api/management.md |
| DELETE | `/api/v1/rules` | 全ルール削除 | [詳細](api/management.md#delete-apiv1rules) @api/management.md |

---

## 技術的決定事項

| ID | 決定内容 | ステータス | 詳細リンク |
|----|---------|-----------|-----------|
| DEC-001 | フォワードプロキシライブラリとして elazarl/goproxy を採用 | 承認済 | [詳細](decisions/DEC-001.md) @decisions/DEC-001.md |
| DEC-002 | HTTP ルーティングに Go 1.22 標準 net/http を使用 | 承認済 | [詳細](decisions/DEC-002.md) @decisions/DEC-002.md |
| DEC-003 | ルールの永続化を行わない（メモリのみ） | 承認済 | [詳細](decisions/DEC-003.md) @decisions/DEC-003.md |

---

## セキュリティ考慮事項

- **デフォルト拒否**: 未登録 IP からの全通信を 403 で拒否（NFR-SEC-001）
- **TLS 非終端**: CONNECT トンネル後はバイパイプのみ。証明書生成・置換は行わない（NFR-SEC-002）
- **ホスト正規化**: `strings.ToLower` + 末尾ドット除去でバイパス攻撃を防止（NFR-SEC-004）
- **競合防止**: `sync.RWMutex` で RuleStore を保護。`go test -race` で検証（NFR-SEC-005）
- **API 露出防止**: Management API（:8080）は internal network のみに公開（NFR-SEC-003）

## パフォーマンス考慮事項

- ルール照合は O(n)（n = エントリ数）。通常の運用では数十エントリ程度のため問題なし
- アクティブ接続数は `atomic.Int64` で管理（mutex 不要）
- `sync.RWMutex` の `RLock` により読み取りは並行可能

## エラー処理戦略

| 状況 | 処理 |
|------|------|
| 送信元 IP 未登録 | 403 + `X-Filter-Reason: no-rules` |
| ホワイトリスト不一致 | 403 + `X-Filter-Reason: denied` |
| 宛先接続失敗 | 502 Bad Gateway |
| プロキシ内部エラー | 500 Internal Server Error |
| API バリデーションエラー | 400 + `validation_error` JSON |
| API 対象未登録 | 404 + `not_found` JSON |

---

## CI/CD 設計

### 品質ゲート

| 項目 | 基準値 | 採用ツール |
|------|--------|-----------|
| テストカバレッジ | 80% 以上 | `go test -cover ./...` |
| Linter | エラー 0 件 | `golangci-lint run` |
| 競合検出 | 0 件 | `go test -race ./...` |
| コード複雑性 | 循環的複雑度 10 以下 | `cyclop`（golangci-lint） |

### Docker ビルド

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /filter-proxy ./cmd/filter-proxy

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /filter-proxy /filter-proxy
EXPOSE 3128 8080
ENTRYPOINT ["/filter-proxy"]
```

**注意**: `gcr.io/distroless/static:nonroot` を使用して非 root ユーザーで実行する。distroless イメージには `curl` が存在しないため `HEALTHCHECK` 命令は使用しない。ヘルスチェックは Docker Compose / Kubernetes の外部プローブに委ねる（US-005）。

---

## 要件との整合性チェック

| 要件 ID | 対応する設計要素 | 状態 |
|--------|----------------|------|
| REQ-001-001〜005 | ProxyHandler（goproxy + CONNECT） | 対応済 |
| REQ-002-001〜010 | Matcher（完全一致・ワイルドカード・IP・CIDR） | 対応済 |
| REQ-003-001〜004 | RuleStore（Get/デフォルト拒否） | 対応済 |
| REQ-004-001〜014 | APIHandler（CRUD・バリデーション・bind 制限） | 対応済 |
| REQ-005-001〜003 | APIHandler（/health エンドポイント） | 対応済 |
| REQ-006-001〜006 | Logger（slog・接続ログ・操作ログ） | 対応済 |
| REQ-007-001〜005 | Server（signal.NotifyContext + Shutdown） | 対応済 |
| NFR-PERF-001〜003 | atomic.Int64 接続数・O(n) マッチング | 対応済 |
| NFR-SEC-001〜005 | デフォルト拒否・TLS 非終端・正規化・RWMutex | 対応済 |
| NFR-MNT-001〜004 | テスト・lint・distroless・Graceful shutdown | 対応済 |
