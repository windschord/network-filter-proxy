# APIHandler コンポーネント

## 概要

**目的**: `:8080` で Management REST API を提供し、ルールセットの動的な CRUD 操作を処理する

**責務**:
- `GET /api/v1/rules` — 全ルール一覧返却
- `PUT /api/v1/rules/{sourceIP}` — ルールセット全置換（バリデーション含む）
- `DELETE /api/v1/rules/{sourceIP}` — 指定 IP のルール削除
- `DELETE /api/v1/rules` — 全ルール削除
- `GET /api/v1/health` — ヘルスチェック
- リクエストのバリデーション
- JSON レスポンス返却
- ルール変更操作のログ出力

---

## インターフェース（Go）

### パッケージ: `internal/api`

```go
// Handler は Management API の HTTP ハンドラー
type Handler struct {
    store        *rule.Store
    logger       *slog.Logger
    proxyHandler *proxy.Handler  // アクティブ接続数・起動時刻取得用
    startTime    time.Time
}

// NewHandler は Handler を生成する
func NewHandler(
    store *rule.Store,
    logger *slog.Logger,
    proxyHandler *proxy.Handler,
) *Handler

// Routes は net/http.ServeMux にルーティングを登録したハンドラーを返す
func (h *Handler) Routes() http.Handler
```

---

## ルーティング設計

Go 1.22 以降の標準 `net/http` のパスパターンマッチングを使用する（`gorilla/mux` 等の追加ライブラリ不要）。

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /api/v1/health",          h.handleHealth)
mux.HandleFunc("GET /api/v1/rules",           h.handleGetRules)
mux.HandleFunc("PUT /api/v1/rules/{sourceIP}", h.handlePutRules)
mux.HandleFunc("DELETE /api/v1/rules/{sourceIP}", h.handleDeleteRulesByIP)
mux.HandleFunc("DELETE /api/v1/rules",        h.handleDeleteAllRules)
```

---

## 各エンドポイントの処理フロー

### PUT /api/v1/rules/{sourceIP}

```text
1. sourceIP = r.PathValue("sourceIP") で取得
2. json.Decode でリクエストボディをパース
3. 各 Entry を rule.ValidateEntry() でバリデーション
   → エラーあり: 400 + validation_error JSON
4. store.Set(sourceIP, entries) でルール更新
5. ログ出力（操作種別: put_rules, 対象 IP, エントリ数）
6. 200 + 更新後のルールセット JSON
```

### DELETE /api/v1/rules/{sourceIP}

```text
1. sourceIP = r.PathValue("sourceIP") で取得
2. store.Delete(sourceIP)
   → false（存在しない）: 404 + not_found JSON
3. ログ出力（操作種別: delete_rules, 対象 IP）
4. 204 No Content
```

### DELETE /api/v1/rules

```text
1. store.DeleteAll()
2. ログ出力（操作種別: delete_all_rules）
3. 204 No Content
```

---

## レスポンス JSON 構造

### GET /api/v1/rules — 200

```go
type GetRulesResponse struct {
    Rules map[string]*RuleSetJSON `json:"rules"`
}

type RuleSetJSON struct {
    Entries []EntryJSON `json:"entries"`
}

type EntryJSON struct {
    Host string `json:"host"`
    Port int    `json:"port,omitempty"`
}
```

### PUT /api/v1/rules/{sourceIP} — 200

```go
type PutRulesResponse struct {
    SourceIP  string      `json:"source_ip"`
    Entries   []EntryJSON `json:"entries"`
    UpdatedAt time.Time   `json:"updated_at"`
}
```

### エラーレスポンス — 400 / 404

```go
type ErrorResponse struct {
    Error   string        `json:"error"`
    Message string        `json:"message"`
    Details []ErrorDetail `json:"details,omitempty"`
}

type ErrorDetail struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

---

## ヘルスチェックレスポンス

```go
type HealthResponse struct {
    Status            string `json:"status"`
    UptimeSeconds     int64  `json:"uptime_seconds"`
    ActiveConnections int64  `json:"active_connections"`
    RuleCount         int    `json:"rule_count"`
}
```

---

## エラー処理

| エラー種別 | 発生条件 | HTTPステータス | error フィールド |
|-----------|---------|--------------|----------------|
| JSON パースエラー | 不正な JSON ボディ | 400 | `bad_request` |
| バリデーションエラー | 無効なホストパターン等 | 400 | `validation_error` |
| 未登録 IP | DELETE 対象が存在しない | 404 | `not_found` |

---

## テスト観点

- [ ] PUT: 正常なルールセットで 200 が返る
- [ ] PUT: `*.*.example.com` で 400 が返る
- [ ] PUT: 空 host で 400 が返る
- [ ] PUT: port=99999 で 400 が返る
- [ ] DELETE /{sourceIP}: 存在する IP で 204 が返る
- [ ] DELETE /{sourceIP}: 存在しない IP で 404 が返る
- [ ] DELETE /rules: 全ルール削除で 204 が返る
- [ ] GET /rules: 登録済みルールが JSON で返る
- [ ] GET /health: 正常時に 200 + `status: "ok"` が返る

## 関連要件

- [US-004](../../requirements/stories/US-004.md) @../../requirements/stories/US-004.md: Management REST API
- [US-005](../../requirements/stories/US-005.md) @../../requirements/stories/US-005.md: ヘルスチェック
- [US-006](../../requirements/stories/US-006.md) @../../requirements/stories/US-006.md: ルール変更操作のログ
