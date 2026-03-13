# TASK-008: Config に API_BIND_ADDR を追加（バリデーション付き）

## 概要

`internal/config/config.go` に `APIBindAddr` フィールドを追加し、`net.ParseIP` でバリデーションを行う。不正値は `127.0.0.1` にフォールバック。

## 対応する要件・設計

- 要件: [US-008](../../requirements/stories/US-008.md) @../../requirements/stories/US-008.md (REQ-008-001〜003)
- 設計: [DEC-004](../../design/decisions/DEC-004.md) @../../design/decisions/DEC-004.md
- 設計: [Server](../../design/components/server.md) @../../design/components/server.md

## 情報の明確性

| 分類 | 内容 |
|------|------|
| 明示された情報 | 環境変数名 `API_BIND_ADDR`、デフォルト `127.0.0.1`、不正値はフォールバック |
| 不明/要確認の情報 | なし |

## 対象ファイル

- `internal/config/config.go` -- `APIBindAddr` フィールドの追加、`net.ParseIP` によるバリデーション
- `internal/config/config_test.go` -- テストケース追加

## 技術的文脈

- 言語: Go 1.26
- バリデーション: `net.ParseIP()` で IP アドレスの妥当性を検証
- 参照すべき既存コード: `internal/config/config.go`（既存の `Load()` 関数パターン）

## 実装手順（TDD）

### 1. テスト作成: `internal/config/config_test.go` に追加

```go
func TestLoad_APIBindAddr_Default(t *testing.T) {
    t.Setenv("API_BIND_ADDR", "")
    // 他の環境変数もクリア
    cfg := config.Load()
    if cfg.APIBindAddr != "127.0.0.1" {
        t.Errorf("APIBindAddr = %q, want %q", cfg.APIBindAddr, "127.0.0.1")
    }
}

func TestLoad_APIBindAddr_AllInterfaces(t *testing.T) {
    t.Setenv("API_BIND_ADDR", "0.0.0.0")
    cfg := config.Load()
    if cfg.APIBindAddr != "0.0.0.0" {
        t.Errorf("APIBindAddr = %q, want %q", cfg.APIBindAddr, "0.0.0.0")
    }
}

func TestLoad_APIBindAddr_SpecificIP(t *testing.T) {
    t.Setenv("API_BIND_ADDR", "172.20.0.2")
    cfg := config.Load()
    if cfg.APIBindAddr != "172.20.0.2" {
        t.Errorf("APIBindAddr = %q, want %q", cfg.APIBindAddr, "172.20.0.2")
    }
}

func TestLoad_APIBindAddr_InvalidFallback(t *testing.T) {
    t.Setenv("API_BIND_ADDR", "abc")
    cfg := config.Load()
    if cfg.APIBindAddr != "127.0.0.1" {
        t.Errorf("APIBindAddr = %q, want %q (fallback)", cfg.APIBindAddr, "127.0.0.1")
    }
}

func TestLoad_APIBindAddr_InvalidIP(t *testing.T) {
    t.Setenv("API_BIND_ADDR", "999.999.999.999")
    cfg := config.Load()
    if cfg.APIBindAddr != "127.0.0.1" {
        t.Errorf("APIBindAddr = %q, want %q (fallback)", cfg.APIBindAddr, "127.0.0.1")
    }
}
```

### 2. テスト実行: 失敗を確認

```bash
go test -race ./internal/config/...
```

### 3. テストコミット

### 4. 実装: `internal/config/config.go`

- `Config` 構造体に `APIBindAddr string` フィールドを追加
- `Load()` 内で `getEnv("API_BIND_ADDR", "127.0.0.1")` を取得
- `net.ParseIP()` で検証し、不正値は `"127.0.0.1"` にフォールバック

### 5. 実装コミット

## 受入基準

- [ ] `Config` 構造体に `APIBindAddr string` フィールドが存在する
- [ ] `API_BIND_ADDR` 未設定時に `"127.0.0.1"` がセットされる
- [ ] `API_BIND_ADDR=0.0.0.0` で `"0.0.0.0"` がセットされる
- [ ] `API_BIND_ADDR=abc` で `"127.0.0.1"` にフォールバックする
- [ ] `API_BIND_ADDR=999.999.999.999` で `"127.0.0.1"` にフォールバックする
- [ ] `go test -race ./internal/config/...` が全テスト通過

## 依存関係

なし（既存コードへのフィールド追加のみ）

## 推定工数

15分（AIエージェント作業時間）

## ステータス

`DONE`
