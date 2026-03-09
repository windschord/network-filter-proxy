# TASK-004: ProxyHandler 実装（HTTP CONNECT フィルタリング）

> **サブエージェント実行指示**
> このドキュメントは、タスク実行エージェントがサブエージェントにそのまま渡すことを想定しています。
> 以下の内容に従って実装を完了してください。

---

## あなたのタスク

**ProxyHandler**（`elazarl/goproxy` ラッパー）を実装してください。HTTP CONNECT リクエストを受け取り、送信元 IP とルールセットに基づいてフィルタリングを行います。

### 実装の目標

`elazarl/goproxy` の CONNECT ハンドラーにフィルタリングロジックを組み込み、許可時は TLS パススルートンネルを確立し、拒否時は `403 Forbidden` + `X-Filter-Reason` ヘッダーを返す。

### 作成/変更するファイル

| 操作 | ファイルパス | 説明 |
|------|-------------|------|
| 作成 | `internal/proxy/handler.go` | ProxyHandler 実装 |
| 作成 | `internal/proxy/handler_test.go` | フィルタリングロジックのテスト |

---

## 技術的コンテキスト

### 使用技術

- 言語: Go 1.24
- ライブラリ: `github.com/elazarl/goproxy`
- テスト: `go test ./internal/proxy/...`

### 参照すべきファイル

- `@internal/rule/store.go` — RuleStore の Get メソッド
- `@internal/rule/matcher.go` — Matches 関数

### 関連する設計書

- `@docs/sdd/design/components/proxy-handler.md` — ProxyHandler の詳細設計・接続判定フロー

### 関連する要件

- `@docs/sdd/requirements/stories/US-001.md` — HTTPS フォワードプロキシ
- `@docs/sdd/requirements/stories/US-002.md` — ホワイトリスト制御
- `@docs/sdd/requirements/stories/US-003.md` — 送信元 IP 識別

---

## 受入基準

- [ ] `internal/proxy/handler.go` が作成されている
- [ ] `Handler` 構造体に `store *rule.Store`、`logger *slog.Logger`、`activeConn atomic.Int64` フィールドが含まれる
- [ ] `NewHandler(store, logger) *Handler` が実装されている
- [ ] `ServeHTTP(w, r)` が実装されている（net/http.Handler インターフェース実装）
- [ ] `ActiveConnections() int64` が実装されている
- [ ] 未登録 IP → `403 Forbidden` + `X-Filter-Reason: no-rules`
- [ ] ホワイトリスト不一致 → `403 Forbidden` + `X-Filter-Reason: denied`
- [ ] 許可時の接続ログが出力される（action=allow）
- [ ] 拒否時の接続ログが出力される（action=deny）
- [ ] `go test ./internal/proxy/...` が全パスする

---

## 実装手順

### ステップ 1: テストを先に作成（TDD）

`internal/proxy/handler_test.go` を作成し、以下のテストケースを実装する:

テストでは `httptest` パッケージを利用してプロキシサーバーを起動し、`http.Client` の `Transport` にプロキシを設定して CONNECT リクエストを送る。

```go
package proxy_test

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/claudework/network-filter-proxy/internal/proxy"
    "github.com/claudework/network-filter-proxy/internal/rule"
    "github.com/claudework/network-filter-proxy/internal/logger"
)

// TestProxyHandler_UnregisteredIP: 未登録 IP → 403 + X-Filter-Reason: no-rules
// TestProxyHandler_AllowedHost: 許可済みルールにマッチ → 200
// TestProxyHandler_DeniedHost: 許可済みルールに不一致 → 403 + X-Filter-Reason: denied
```

テストを実行してコンパイルエラーを確認:
```bash
go test ./internal/proxy/...
```

コミット: `test: Add ProxyHandler unit tests`

### ステップ 2: handler.go を実装

```go
package proxy

import (
    "fmt"
    "log/slog"
    "net"
    "net/http"
    "sync"
    "sync/atomic"

    "github.com/claudework/network-filter-proxy/internal/rule"
    "github.com/elazarl/goproxy"
)

// trackingConn は net.Conn をラップし、Close 時に activeConn をデクリメントする
type trackingConn struct {
    net.Conn
    done      func()
    closeOnce sync.Once
}

func (c *trackingConn) Close() error {
    c.closeOnce.Do(c.done)
    return c.Conn.Close()
}

type Handler struct {
    store      *rule.Store
    logger     *slog.Logger
    activeConn atomic.Int64
    proxy      *goproxy.ProxyHttpServer
}

func NewHandler(store *rule.Store, logger *slog.Logger) *Handler {
    h := &Handler{store: store, logger: logger}
    p := goproxy.NewProxyHttpServer()

    p.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(
        func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
            srcIP := extractIP(ctx.Req.RemoteAddr)
            dstHost, dstPort := splitHostPort(host)

            rs, ok := store.Get(srcIP)
            if !ok {
                logger.Info("proxy request",
                    "action", "deny", "src_ip", srcIP,
                    "dst_host", dstHost, "dst_port", dstPort,
                    "reason", "no-rules")
                ctx.Resp = &http.Response{
                    StatusCode: http.StatusForbidden,
                    Header:     http.Header{"X-Filter-Reason": {"no-rules"}},
                    Body:       http.NoBody,
                }
                return goproxy.RejectConnect, host
            }

            for _, entry := range rs.Entries {
                if rule.Matches(entry, dstHost, dstPort) {
                    logger.Info("proxy request",
                        "action", "allow", "src_ip", srcIP,
                        "dst_host", dstHost, "dst_port", dstPort)
                    // アクティブ接続数をインクリメントし、トンネル終了時にデクリメントする
                    // goproxy.ConnectAction.Dial で trackingConn を返すことで接続終了を検知する
                    h.activeConn.Add(1)
                    return &goproxy.ConnectAction{
                        Action: goproxy.OkConnect,
                        Dial: func(network, addr string) (net.Conn, error) {
                            c, err := net.Dial(network, addr)
                            if err != nil {
                                h.activeConn.Add(-1)
                                return nil, err
                            }
                            return &trackingConn{
                                Conn: c,
                                done: func() { h.activeConn.Add(-1) },
                            }, nil
                        },
                    }, host
                }
            }

            logger.Info("proxy request",
                "action", "deny", "src_ip", srcIP,
                "dst_host", dstHost, "dst_port", dstPort,
                "reason", "denied")
            ctx.Resp = &http.Response{
                StatusCode: http.StatusForbidden,
                Header:     http.Header{"X-Filter-Reason": {"denied"}},
                Body:       http.NoBody,
            }
            return goproxy.RejectConnect, host
        },
    ))

    // 通常 HTTP リクエスト（CONNECT 以外）のフィルタリング
    p.OnRequest().DoFunc(
        func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
            srcIP := extractIP(r.RemoteAddr)
            dstHost, dstPort := splitHostPort(r.Host)

            rs, ok := store.Get(srcIP)
            if !ok {
                logger.Info("proxy request",
                    "action", "deny", "src_ip", srcIP,
                    "dst_host", dstHost, "dst_port", dstPort,
                    "reason", "no-rules")
                return r, &http.Response{
                    StatusCode: http.StatusForbidden,
                    Header:     http.Header{"X-Filter-Reason": {"no-rules"}},
                    Body:       http.NoBody,
                    Request:    r,
                }
            }

            for _, entry := range rs.Entries {
                if rule.Matches(entry, dstHost, dstPort) {
                    logger.Info("proxy request",
                        "action", "allow", "src_ip", srcIP,
                        "dst_host", dstHost, "dst_port", dstPort)
                    return r, nil
                }
            }

            logger.Info("proxy request",
                "action", "deny", "src_ip", srcIP,
                "dst_host", dstHost, "dst_port", dstPort,
                "reason", "denied")
            return r, &http.Response{
                StatusCode: http.StatusForbidden,
                Header:     http.Header{"X-Filter-Reason": {"denied"}},
                Body:       http.NoBody,
                Request:    r,
            }
        },
    )

    h.proxy = p
    return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    h.proxy.ServeHTTP(w, r)
}

func (h *Handler) ActiveConnections() int64 {
    return h.activeConn.Load()
}

func extractIP(remoteAddr string) string {
    host, _, err := net.SplitHostPort(remoteAddr)
    if err != nil {
        return remoteAddr
    }
    return host
}

func splitHostPort(hostport string) (string, int) {
    host, portStr, err := net.SplitHostPort(hostport)
    if err != nil {
        return hostport, 0
    }
    var port int
    fmt.Sscan(portStr, &port)
    return host, port
}
```

### ステップ 3: テストを実行して確認

```bash
go test ./internal/proxy/...
go test -race ./internal/proxy/...
```

### ステップ 4: コミット

```
feat: Implement ProxyHandler with whitelist filtering
```

---

## 注意事項

- `goproxy.RejectConnect` 時のレスポンスは `ctx.Resp` に事前に設定する
- `X-Filter-Reason` ヘッダーは拒否時のみ付与する（`no-rules` または `denied`）
- アクティブ接続数は `atomic.Int64` で管理する（接続開始時に `Add(1)`、切断時に `Add(-1)`）

---

## 基本情報（メタデータ）

| 項目 | 値 |
|------|-----|
| **タスクID** | TASK-004 |
| **ステータス** | `TODO` |
| **推定工数** | 40分 |
| **依存関係** | [TASK-002](../phase-2/TASK-002.md) @../phase-2/TASK-002.md, [TASK-003](../phase-2/TASK-003.md) @../phase-2/TASK-003.md |
| **対応要件** | REQ-001-001〜005, REQ-002-001〜010, REQ-003-001〜004 |
| **対応設計** | components/proxy-handler.md |

---

## 情報の明確性チェック

### 明示された情報

- [x] `elazarl/goproxy` の `FuncHttpsHandler` を使用
- [x] TLS 終端しない（OkConnect = パススルー）
- [x] 未登録 IP: `X-Filter-Reason: no-rules`
- [x] 不一致: `X-Filter-Reason: denied`
