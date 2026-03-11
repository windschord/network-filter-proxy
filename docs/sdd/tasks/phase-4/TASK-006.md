# TASK-006: main.go 統合（DI・Graceful Shutdown）

> **サブエージェント実行指示**
> このドキュメントは、タスク実行エージェントがサブエージェントにそのまま渡すことを想定しています。
> 以下の内容に従って実装を完了してください。

---

## あなたのタスク

**main.go** を実装してください。全コンポーネントを依存注入（DI）し、プロキシサーバーと API サーバーを並行起動し、SIGTERM/SIGINT による Graceful Shutdown を実装します。

### 実装の目標

`cmd/filter-proxy/main.go` を実装して、`Config` 読み込み → `Logger`・`RuleStore`・`ProxyHandler`・`APIHandler` を生成 → 両サーバー起動 → シグナル受信 → `Shutdown()` の完全なライフサイクルを実現する。

### 作成/変更するファイル

| 操作 | ファイルパス | 説明 |
|------|-------------|------|
| 作成 | `cmd/filter-proxy/main.go` | エントリポイント |

---

## 技術的コンテキスト

### 使用技術

- 言語: Go 1.26
- 標準ライブラリ: `os/signal`、`context`、`net/http`、`sync`

### 参照すべきファイル

- `@internal/config/config.go` — Load()
- `@internal/logger/logger.go` — New()
- `@internal/rule/store.go` — NewStore()
- `@internal/proxy/handler.go` — NewHandler()
- `@internal/api/handler.go` — NewHandler(), Routes()

### 関連する設計書

- `@docs/sdd/design/components/server.md` — 起動フロー・Graceful Shutdown フロー

### 関連する要件

- `@docs/sdd/requirements/stories/US-007.md` — Graceful Shutdown

---

## 受入基準

- [ ] `cmd/filter-proxy/main.go` が作成されている
- [ ] `go build ./cmd/filter-proxy` が成功する
- [ ] プロキシサーバー（:3128）と API サーバー（127.0.0.1:8080）が並行起動する
- [ ] `GET http://127.0.0.1:8080/api/v1/health` が 200 を返す
- [ ] SIGTERM 受信後、両サーバーが `SHUTDOWN_TIMEOUT` 秒以内に停止する
- [ ] `go build -o /dev/null ./...` がエラーなし

---

## 実装手順

### ステップ 1: main.go を実装

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "sync"
    "syscall"

    "github.com/claudework/network-filter-proxy/internal/api"
    "github.com/claudework/network-filter-proxy/internal/config"
    "github.com/claudework/network-filter-proxy/internal/logger"
    "github.com/claudework/network-filter-proxy/internal/proxy"
    "github.com/claudework/network-filter-proxy/internal/rule"
)

// run はアプリケーション本体。終了コードを返す。
// os.Exit は defer を実行しないため、defer を使うロジックはすべて run() に収める。
func run() int {
    cfg := config.Load()
    log := logger.New(cfg.LogFormat, cfg.LogLevel)

    store := rule.NewStore()
    proxyHandler := proxy.NewHandler(store, log)
    apiHandler := api.NewHandler(store, log, proxyHandler)

    proxySrv := &http.Server{
        Addr:    ":" + cfg.ProxyPort,
        Handler: proxyHandler,
    }
    apiSrv := &http.Server{
        Addr:    "127.0.0.1:" + cfg.APIPort,
        Handler: apiHandler.Routes(),
    }

    // シグナル待機コンテキストを先に生成する
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer stop()

    // 両サーバーを goroutine で起動。起動失敗はエラーチャネルで run goroutine に伝搬し、
    // os.Exit(1) を goroutine 内で呼ばずに graceful shutdown 経路を通る
    errCh := make(chan error, 2)
    go func() {
        log.Info("proxy server starting", "port", cfg.ProxyPort)
        if err := proxySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Error("proxy server error", "err", err)
            errCh <- err
        }
    }()
    go func() {
        log.Info("api server starting", "addr", "127.0.0.1:"+cfg.APIPort)
        if err := apiSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Error("api server error", "err", err)
            errCh <- err
        }
    }()

    // シグナルまたはサーバーエラーのいずれかを待機
    exitCode := 0
    select {
    case <-ctx.Done():
        // SIGTERM / SIGINT 受信
    case err := <-errCh:
        log.Error("server failed, initiating shutdown", "err", err)
        stop() // コンテキストをキャンセルして graceful shutdown へ移行
        exitCode = 1
    }

    // Graceful Shutdown
    log.Info("shutdown initiated")
    shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
    defer cancel()

    var wg sync.WaitGroup
    for _, srv := range []*http.Server{proxySrv, apiSrv} {
        wg.Add(1)
        go func(s *http.Server) {
            defer wg.Done()
            if err := s.Shutdown(shutdownCtx); err != nil {
                log.Error("shutdown error", "err", err, slog.String("addr", s.Addr))
            }
        }(srv)
    }
    wg.Wait()
    log.Info("shutdown complete")
    return exitCode
}

func main() {
    os.Exit(run())
}
```

### ステップ 2: ビルドを確認

```bash
go build ./cmd/filter-proxy
go build -o /dev/null ./...
```

### ステップ 3: 動作確認

```bash
# バイナリを起動
./filter-proxy &
# ヘルスチェック
curl http://localhost:8080/api/v1/health
# 停止
kill -TERM $(pgrep filter-proxy)
```

### ステップ 4: コミット

```text
feat: Implement main entry point with graceful shutdown
```

---

## 注意事項

- `http.ErrServerClosed` は正常なシャットダウン時に返されるため、エラーとして扱わない
- `signal.NotifyContext` を使うと `context` がキャンセルされた時点でシグナル受信を検知できる
- `sync.WaitGroup` で両サーバーの Shutdown 完了を待つ

---

## 基本情報（メタデータ）

| 項目 | 値 |
|------|-----|
| **タスクID** | TASK-006 |
| **ステータス** | `TODO` |
| **推定工数** | 30分 |
| **依存関係** | [TASK-004](../phase-3/TASK-004.md) @../phase-3/TASK-004.md, [TASK-005](../phase-3/TASK-005.md) @../phase-3/TASK-005.md |
| **対応要件** | REQ-007-001〜005 |
| **対応設計** | components/server.md, DEC-001, DEC-002 |

---

## 情報の明確性チェック

### 明示された情報

- [x] SIGTERM と SIGINT の両方を受け取る
- [x] `Server.Shutdown()` でプロキシ・API サーバー両方をシャットダウン
- [x] `SHUTDOWN_TIMEOUT` 秒以内に強制終了
