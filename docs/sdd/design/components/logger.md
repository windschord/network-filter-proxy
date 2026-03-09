# Logger コンポーネント

## 概要

**目的**: `log/slog` を使って構造化ログ（JSON / テキスト形式）を stdout へ出力する

**責務**:
- JSON または テキスト形式の構造化ログ出力
- ログレベルフィルタリング（debug / info / warn / error）
- 環境変数 `LOG_LEVEL` / `LOG_FORMAT` による設定

---

## インターフェース（Go）

### パッケージ: `internal/logger`

```go
// New は設定に基づいて *slog.Logger を生成する
// format: "json" または "text"（デフォルト: "json"）
// level:  "debug" / "info" / "warn" / "error"（デフォルト: "info"）
func New(format, level string) *slog.Logger
```

---

## ログフィールド定義

### 接続ログ（ProxyHandler が出力）

```go
logger.Info("proxy request",
    "action",    "allow" | "deny",
    "src_ip",    sourceIP,
    "dst_host",  host,
    "dst_port",  port,
    "reason",    "no-rules" | "denied" | "",  // 拒否時のみ
)
```

### ルール変更ログ（APIHandler が出力）

```go
logger.Info("rule change",
    "operation", "put_rules" | "delete_rules" | "delete_all_rules",
    "src_ip",    sourceIP,   // delete_all_rules 時は省略
    "count",     len(entries),
)
```

### Graceful shutdown ログ

```go
logger.Info("shutdown initiated")
logger.Info("shutdown complete")
```

---

## 設定マッピング

| 環境変数 | 値 | slog 設定 |
|---------|-----|----------|
| `LOG_FORMAT=json` | デフォルト | `slog.NewJSONHandler(os.Stdout, opts)` |
| `LOG_FORMAT=text` | - | `slog.NewTextHandler(os.Stdout, opts)` |
| `LOG_LEVEL=debug` | - | `slog.LevelDebug` |
| `LOG_LEVEL=info` | デフォルト | `slog.LevelInfo` |
| `LOG_LEVEL=warn` | - | `slog.LevelWarn` |
| `LOG_LEVEL=error` | - | `slog.LevelError` |

---

## 依存関係

### 依存するコンポーネント
- Go 標準ライブラリ `log/slog`（Go 1.21+）

### 依存されるコンポーネント
- [proxy-handler](proxy-handler.md) @proxy-handler.md
- [api-handler](api-handler.md) @api-handler.md

---

## テスト観点

- [ ] `LOG_FORMAT=json` で JSON 形式ログが出力される
- [ ] `LOG_FORMAT=text` でテキスト形式ログが出力される
- [ ] `LOG_LEVEL=warn` で info ログが出力されない

## 関連要件

- [US-006](../../requirements/stories/US-006.md) @../../requirements/stories/US-006.md: 構造化ログ出力
