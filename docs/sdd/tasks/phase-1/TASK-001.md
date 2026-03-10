# TASK-001: プロジェクト基盤構築（go.mod / config / logger）

> **サブエージェント実行指示**
> このドキュメントは、タスク実行エージェントがサブエージェントにそのまま渡すことを想定しています。
> 以下の内容に従って実装を完了してください。

---

## あなたのタスク

**プロジェクト初期化・設定読み込み・ロガーファクトリ** を実装してください。

### 実装の目標

Go モジュールを初期化し、環境変数から設定を読み込む `config` パッケージと、`log/slog` ベースの構造化ロガーを生成する `logger` パッケージを実装する。これらは全コンポーネントの基盤となる。

### 作成/変更するファイル

| 操作 | ファイルパス | 説明 |
|------|-------------|------|
| 作成 | `go.mod` | Go 1.26 モジュール定義・依存ライブラリ宣言 |
| 作成 | `go.sum` | 依存ライブラリのチェックサム（`go mod tidy` で生成） |
| 作成 | `internal/config/config.go` | 環境変数読み込み・Config 構造体 |
| 作成 | `internal/config/config_test.go` | config のユニットテスト |
| 作成 | `internal/logger/logger.go` | slog.Logger ファクトリ |
| 作成 | `internal/logger/logger_test.go` | logger のユニットテスト |

---

## 技術的コンテキスト

### 使用技術

- 言語: Go 1.26
- テスト: `go test ./...`
- 外部ライブラリ: `github.com/elazarl/goproxy`（go.mod に宣言するのみ。本タスクでは使用しない）

### 関連する設計書

- `@docs/sdd/design/components/server.md` — Config 構造体・環境変数一覧
- `@docs/sdd/design/components/logger.md` — Logger ファクトリ仕様

### 関連する要件

- `@docs/sdd/requirements/stories/US-006.md` — 構造化ログ出力

---

## 受入基準

- [ ] `go.mod` が存在し、モジュール名 `github.com/claudework/network-filter-proxy`、Go 1.26 が宣言されている
- [ ] `go.mod` に `github.com/elazarl/goproxy` が依存ライブラリとして追加されている
- [ ] `internal/config/config.go` に `Config` 構造体と `Load() Config` 関数が実装されている
- [ ] 環境変数未設定時にデフォルト値（ProxyPort=3128, APIPort=8080, LogLevel=info, LogFormat=json, ShutdownTimeout=30s）が使用される
- [ ] `internal/logger/logger.go` に `New(format, level string) *slog.Logger` が実装されている
- [ ] `LOG_FORMAT=json` で JSON ハンドラー、`LOG_FORMAT=text` でテキストハンドラーが返される
- [ ] `go test ./internal/config/... ./internal/logger/...` が全パスする
- [ ] `go test -race ./internal/config/... ./internal/logger/...` が全パスする

---

## 実装手順

### ステップ 1: テストを先に作成（TDD）

1. `internal/config/config_test.go` を作成し、以下のテストケースを実装する:
   - 環境変数未設定時にデフォルト値が返る
   - `PROXY_PORT=9999` 設定時に `Config.ProxyPort == "9999"` になる
   - `SHUTDOWN_TIMEOUT=60` 設定時に `Config.ShutdownTimeout == 60*time.Second` になる
   - `SHUTDOWN_TIMEOUT=abc`（パース失敗）時に `Config.ShutdownTimeout == 30*time.Second` になる
2. `internal/logger/logger_test.go` を作成し、以下のテストケースを実装する:
   - `New("json", "info")` で非 nil の `*slog.Logger` が返る
   - `New("text", "debug")` で非 nil の `*slog.Logger` が返る
3. `go test ./...` を実行してコンパイルエラー（テスト失敗）を確認
4. コミット: `test: Add tests for config and logger`

### ステップ 2: go.mod を初期化

```bash
go mod init github.com/claudework/network-filter-proxy
go get github.com/elazarl/goproxy
go mod tidy
```

### ステップ 3: config パッケージを実装

`internal/config/config.go` に以下を実装:

```go
package config

import (
    "os"
    "strconv"
    "time"
)

type Config struct {
    ProxyPort       string
    APIPort         string
    LogLevel        string
    LogFormat       string
    ShutdownTimeout time.Duration
}

func Load() Config {
    timeout := 30
    if v, err := strconv.Atoi(getEnv("SHUTDOWN_TIMEOUT", "30")); err == nil {
        timeout = v
    }
    return Config{
        ProxyPort:       getEnv("PROXY_PORT", "3128"),
        APIPort:         getEnv("API_PORT", "8080"),
        LogLevel:        getEnv("LOG_LEVEL", "info"),
        LogFormat:       getEnv("LOG_FORMAT", "json"),
        ShutdownTimeout: time.Duration(timeout) * time.Second,
    }
}

func getEnv(key, defaultValue string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultValue
}
```

### ステップ 4: logger パッケージを実装

`internal/logger/logger.go` に以下を実装:

```go
package logger

import (
    "log/slog"
    "os"
    "strings"
)

func New(format, level string) *slog.Logger {
    var lvl slog.Level
    switch strings.ToLower(level) {
    case "debug":
        lvl = slog.LevelDebug
    case "warn":
        lvl = slog.LevelWarn
    case "error":
        lvl = slog.LevelError
    default:
        lvl = slog.LevelInfo
    }
    opts := &slog.HandlerOptions{
        Level: lvl,
        ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
            if len(groups) == 0 && a.Key == slog.TimeKey {
                a.Key = "timestamp" // "time" → "timestamp" (US-006 要件)
            }
            return a
        },
    }
    var handler slog.Handler
    if strings.ToLower(format) == "text" {
        handler = slog.NewTextHandler(os.Stdout, opts)
    } else {
        handler = slog.NewJSONHandler(os.Stdout, opts)
    }
    return slog.New(handler)
}
```

### ステップ 5: テストを実行して確認

```bash
go test ./internal/config/... ./internal/logger/...
go test -race ./internal/config/... ./internal/logger/...
```

### ステップ 6: コミットして完了

```text
feat: Implement config and logger packages
```

---

## 実装の詳細仕様

### 環境変数一覧

| 変数名 | デフォルト | 型 | 説明 |
|--------|-----------|-----|------|
| `PROXY_PORT` | `"3128"` | string | プロキシリッスンポート |
| `API_PORT` | `"8080"` | string | API リッスンポート |
| `LOG_LEVEL` | `"info"` | string | `debug`/`info`/`warn`/`error` |
| `LOG_FORMAT` | `"json"` | string | `json`/`text` |
| `SHUTDOWN_TIMEOUT` | `"30"` | int 秒 → time.Duration | Graceful shutdown 待機時間 |

### ログレベルマッピング

| 文字列 | slog.Level |
|--------|------------|
| `"debug"` | `slog.LevelDebug` |
| `"info"` | `slog.LevelInfo`（デフォルト） |
| `"warn"` | `slog.LevelWarn` |
| `"error"` | `slog.LevelError` |

---

## 注意事項

- `go.mod` のモジュール名は `github.com/claudework/network-filter-proxy` を使用する
- `SHUTDOWN_TIMEOUT` のパース失敗時は 30 秒をデフォルトとする
- ログ出力先は `os.Stdout` 固定

---

## 基本情報（メタデータ）

| 項目 | 値 |
|------|-----|
| **タスクID** | TASK-001 |
| **ステータス** | `TODO` |
| **推定工数** | 20分 |
| **依存関係** | なし |
| **対応要件** | REQ-006-004〜006 |
| **対応設計** | server.md, logger.md |

---

## 情報の明確性チェック

### 明示された情報

- [x] Go 1.26 を使用
- [x] モジュール名: `github.com/claudework/network-filter-proxy`
- [x] 環境変数名・デフォルト値が仕様に明記されている
- [x] ログ形式: JSON（デフォルト）/ テキスト

### 未確認/要確認の情報

なし
