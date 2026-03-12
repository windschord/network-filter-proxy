# TASK-009: healthcheck サブコマンド + API バインドアドレス適用

## 概要

`cmd/filter-proxy/main.go` に以下の変更を加える:
1. `healthcheck` サブコマンドの実装（`os.Args[1] == "healthcheck"` で分岐）
2. API サーバーのバインドアドレスを `cfg.APIBindAddr` から構築

## 対応する要件・設計

- 要件: [US-008](../../requirements/stories/US-008.md) @../../requirements/stories/US-008.md (REQ-008-001, REQ-008-004)
- 要件: [US-009](../../requirements/stories/US-009.md) @../../requirements/stories/US-009.md (REQ-009-001〜005)
- 設計: [DEC-005](../../design/decisions/DEC-005.md) @../../design/decisions/DEC-005.md
- 設計: [Server](../../design/components/server.md) @../../design/components/server.md

## 情報の明確性

| 分類 | 内容 |
|------|------|
| 明示された情報 | healthcheck は `127.0.0.1:{API_PORT}` に GET、タイムアウト 5 秒、exit 0/1 |
| 不明/要確認の情報 | なし |

## 対象ファイル

- `cmd/filter-proxy/main.go` -- healthcheck サブコマンド追加、API バインドアドレス適用
- `cmd/filter-proxy/main_test.go` -- healthcheck のテスト（新規作成）

## 技術的文脈

- 言語: Go 1.26
- healthcheck は `config.Load()` を使わず、`os.Getenv("API_PORT")` で軽量に取得
- `http.Client{Timeout: 5 * time.Second}` でタイムアウト制御
- 参照すべき既存コード: `cmd/filter-proxy/main.go`（既存の `run()` 関数パターン）

## 実装手順（TDD）

### 1. テスト作成: `cmd/filter-proxy/main_test.go`（新規）

```go
package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRunHealthcheck_Success(t *testing.T) {
    // /api/v1/health で 200 を返すテストサーバー
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/api/v1/health" {
            w.WriteHeader(http.StatusOK)
            return
        }
        w.WriteHeader(http.StatusNotFound)
    }))
    defer srv.Close()

    // テストサーバーのポートを抽出して API_PORT に設定
    // srv.Listener.Addr() からポートを取得
    t.Setenv("API_PORT", extractPort(srv))

    code := runHealthcheck()
    if code != 0 {
        t.Errorf("runHealthcheck() = %d, want 0", code)
    }
}

func TestRunHealthcheck_ServerDown(t *testing.T) {
    // 存在しないポートを指定
    t.Setenv("API_PORT", "19999")

    code := runHealthcheck()
    if code != 1 {
        t.Errorf("runHealthcheck() = %d, want 1", code)
    }
}

func TestRunHealthcheck_Non200(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusServiceUnavailable)
    }))
    defer srv.Close()

    t.Setenv("API_PORT", extractPort(srv))

    code := runHealthcheck()
    if code != 1 {
        t.Errorf("runHealthcheck() = %d, want 1", code)
    }
}
```

### 2. テスト実行: 失敗を確認

```bash
go test -race ./cmd/filter-proxy/...
```

### 3. テストコミット

### 4. 実装: `cmd/filter-proxy/main.go`

**healthcheck サブコマンド:**

```go
func main() {
    if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
        os.Exit(runHealthcheck())
    }
    os.Exit(run())
}

func runHealthcheck() int {
    port := os.Getenv("API_PORT")
    if port == "" {
        port = "8080"
    }
    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Get("http://127.0.0.1:" + port + "/api/v1/health")
    if err != nil {
        return 1
    }
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusOK {
        return 0
    }
    return 1
}
```

**API バインドアドレス適用:**

```go
// 変更前
apiSrv := &http.Server{
    Addr:    "127.0.0.1:" + cfg.APIPort,
    Handler: apiHandler.Routes(),
}

// 変更後
apiSrv := &http.Server{
    Addr:    cfg.APIBindAddr + ":" + cfg.APIPort,
    Handler: apiHandler.Routes(),
}
```

ログ出力も更新:

```go
// 変更前
log.Info("api server starting", "addr", "127.0.0.1:"+cfg.APIPort)

// 変更後
log.Info("api server starting", "addr", cfg.APIBindAddr+":"+cfg.APIPort)
```

### 5. 実装コミット

## 受入基準

- [ ] `filter-proxy healthcheck` で API が 200 の場合に終了コード 0 を返す
- [ ] `filter-proxy healthcheck` で API 応答なしの場合に終了コード 1 を返す
- [ ] `filter-proxy healthcheck` で API が非 200 の場合に終了コード 1 を返す
- [ ] healthcheck の HTTP タイムアウトが 5 秒である
- [ ] API サーバーが `cfg.APIBindAddr:cfg.APIPort` にバインドされる
- [ ] 起動ログに実際のバインドアドレスが出力される
- [ ] `go test -race ./cmd/filter-proxy/...` が全テスト通過

## 依存関係

TASK-008（Config の APIBindAddr フィールドが必要）

## 推定工数

25分（AIエージェント作業時間）

## ステータス

`TODO`
