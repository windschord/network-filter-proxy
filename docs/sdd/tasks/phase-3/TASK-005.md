# TASK-005: APIHandler 実装（Management REST API）

> **サブエージェント実行指示**
> このドキュメントは、タスク実行エージェントがサブエージェントにそのまま渡すことを想定しています。
> 以下の内容に従って実装を完了してください。

---

## あなたのタスク

**APIHandler**（Management REST API）を実装してください。ルールセットの CRUD 操作とヘルスチェックエンドポイントを提供します。

### 実装の目標

Go 1.22 以降の標準 `net/http` ServeMux を使ってルーティングを実装し、`RuleStore` に対する CRUD 操作・バリデーション・JSON レスポンスを処理する。

### 作成/変更するファイル

| 操作 | ファイルパス | 説明 |
|------|-------------|------|
| 作成 | `internal/api/handler.go` | APIHandler 実装 |
| 作成 | `internal/api/handler_test.go` | 全エンドポイントのテスト |

---

## 技術的コンテキスト

### 使用技術

- 言語: Go 1.26
- ルーティング: 標準 `net/http` ServeMux（Go 1.22 拡張）
- JSON: `encoding/json`
- テスト: `net/http/httptest`

### 参照すべきファイル

- `@internal/rule/store.go` — RuleStore の CRUD メソッド
- `@internal/rule/matcher.go` — ValidateEntry 関数

### 関連する設計書

- `@docs/sdd/design/components/api-handler.md` — APIHandler の詳細設計・JSON 構造
- `@docs/sdd/design/api/management.md` — 全エンドポイント仕様

### 関連する要件

- `@docs/sdd/requirements/stories/US-004.md` — Management REST API
- `@docs/sdd/requirements/stories/US-005.md` — ヘルスチェック

---

## 受入基準

- [ ] `internal/api/handler.go` が作成されている
- [ ] `GET /api/v1/health` → 200 + `{"status":"ok",...}` が返る
- [ ] `GET /api/v1/rules` → 200 + 全ルールの JSON が返る
- [ ] `PUT /api/v1/rules/{sourceIP}` → 200 + 更新後ルール JSON が返る
- [ ] `PUT /api/v1/rules/{sourceIP}` でバリデーションエラー → 400 + `validation_error` JSON
- [ ] `DELETE /api/v1/rules/{sourceIP}` 存在する IP → 204
- [ ] `DELETE /api/v1/rules/{sourceIP}` 存在しない IP → 404 + `not_found` JSON
- [ ] `DELETE /api/v1/rules` → 204
- [ ] ルール変更操作時にログが出力される
- [ ] `go test ./internal/api/...` が全パスする

---

## 実装手順

### ステップ 1: テストを先に作成（TDD）

`internal/api/handler_test.go` を作成し、`httptest.NewRecorder` を使ったテストを実装する:

```go
package api_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/claudework/network-filter-proxy/internal/api"
    "github.com/claudework/network-filter-proxy/internal/rule"
    "github.com/claudework/network-filter-proxy/internal/logger"
)

func TestHealth(t *testing.T) { ... }          // 200 + status=ok
func TestGetRules_Empty(t *testing.T) { ... }  // 200 + {"rules":{}}
func TestPutRules_Valid(t *testing.T) { ... }  // 200 + updated rules
func TestPutRules_ValidationError(t *testing.T) { ... }  // 400
func TestDeleteRulesByIP_Exists(t *testing.T) { ... }    // 204
func TestDeleteRulesByIP_NotFound(t *testing.T) { ... }  // 404
func TestDeleteAllRules(t *testing.T) { ... }            // 204
```

テストを実行してコンパイルエラーを確認:
```bash
go test ./internal/api/...
```

コミット: `test: Add APIHandler unit tests`

### ステップ 2: handler.go を実装

#### 型定義（JSON 構造）

```go
package api

import (
    "encoding/json"
    "log/slog"
    "net/http"
    "time"

    "github.com/claudework/network-filter-proxy/internal/proxy"
    "github.com/claudework/network-filter-proxy/internal/rule"
)

type Handler struct {
    store        *rule.Store
    logger       *slog.Logger
    proxyHandler *proxy.Handler
    startTime    time.Time
}

type entryJSON struct {
    Host string `json:"host"`
    Port int    `json:"port,omitempty"`
}

type ruleSetJSON struct {
    Entries []entryJSON `json:"entries"`
}

type putRulesRequest struct {
    Entries []entryJSON `json:"entries"`
}

type putRulesResponse struct {
    SourceIP  string      `json:"source_ip"`
    Entries   []entryJSON `json:"entries"`
    UpdatedAt time.Time   `json:"updated_at"`
}

type errorResponse struct {
    Error   string        `json:"error"`
    Message string        `json:"message"`
    Details []errorDetail `json:"details,omitempty"`
}

type errorDetail struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

type healthResponse struct {
    Status            string `json:"status"`
    UptimeSeconds     int64  `json:"uptime_seconds"`
    ActiveConnections int64  `json:"active_connections"`
    RuleCount         int    `json:"rule_count"`
}
```

#### ルーティング登録（Go 1.22 ServeMux 拡張）

```go
func (h *Handler) Routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /api/v1/health",               h.handleHealth)
    mux.HandleFunc("GET /api/v1/rules",                h.handleGetRules)
    mux.HandleFunc("PUT /api/v1/rules/{sourceIP}",     h.handlePutRules)
    mux.HandleFunc("DELETE /api/v1/rules/{sourceIP}",  h.handleDeleteRulesByIP)
    mux.HandleFunc("DELETE /api/v1/rules",             h.handleDeleteAllRules)
    return mux
}
```

#### 各ハンドラーの実装骨格

- `handleHealth`: `store.Count()` と `proxyHandler.ActiveConnections()` を使って JSON 返却
- `handleGetRules`: `store.All()` を JSON 変換して返却
- `handlePutRules`: `r.PathValue("sourceIP")` でパス取得 → JSON デコード → ValidateEntry → Set → ログ → 200
- `handleDeleteRulesByIP`: `r.PathValue("sourceIP")` → Delete → 存在しない場合 404 → ログ → 204
- `handleDeleteAllRules`: `store.DeleteAll()` → ログ → 204

### ステップ 3: テストを実行して確認

```bash
go test ./internal/api/...
```

### ステップ 4: コミット

```
feat: Implement APIHandler with full CRUD endpoints
```

---

## API レスポンス仕様

### PUT /api/v1/rules/{sourceIP} — 200

```json
{
  "source_ip": "172.20.0.3",
  "entries": [
    {"host": "api.anthropic.com", "port": 443},
    {"host": "*.npmjs.org", "port": 443}
  ],
  "updated_at": "2026-03-09T10:30:00Z"
}
```

### PUT バリデーションエラー — 400

```json
{
  "error": "validation_error",
  "message": "invalid host pattern: **.example.com",
  "details": [
    {"field": "entries[0].host", "message": "invalid wildcard pattern"}
  ]
}
```

### DELETE /{sourceIP} 404

```json
{
  "error": "not_found",
  "message": "no rules found for source IP: 172.20.0.99"
}
```

---

## 注意事項

- Go 1.22 以降の `r.PathValue("sourceIP")` を使用する（`gorilla/mux` 不要）
- `port=0` のエントリは JSON レスポンスで `port` フィールドを省略する（`omitempty`）
- ルール変更操作のログには `operation`、`src_ip`、エントリ数を含める

---

## 基本情報（メタデータ）

| 項目 | 値 |
|------|-----|
| **タスクID** | TASK-005 |
| **ステータス** | `TODO` |
| **推定工数** | 45分 |
| **依存関係** | [TASK-002](../phase-2/TASK-002.md) @../phase-2/TASK-002.md, [TASK-003](../phase-2/TASK-003.md) @../phase-2/TASK-003.md |
| **対応要件** | REQ-004-001〜013, REQ-005-001〜003 |
| **対応設計** | components/api-handler.md, api/management.md, DEC-002 |

---

## 情報の明確性チェック

### 明示された情報

- [x] Go 1.22 標準 ServeMux の `r.PathValue()` を使用
- [x] 認証なし
- [x] JSON レスポンス（Content-Type: application/json）
- [x] `port=0` は `omitempty` で省略
