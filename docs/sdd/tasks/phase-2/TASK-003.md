# TASK-003: Matcher 実装（ホスト・ポートマッチング + バリデーション）

> **サブエージェント実行指示**
> このドキュメントは、タスク実行エージェントがサブエージェントにそのまま渡すことを想定しています。
> 以下の内容に従って実装を完了してください。

---

## あなたのタスク

**Matcher（ホストパターンマッチング + ルールバリデーション）** を実装してください。

### 実装の目標

ルールエントリ（ホストパターン + ポート）と接続先（ホスト + ポート）のマッチング判定関数 `Matches`、およびルールエントリのバリデーション関数 `ValidateEntry` を実装する。完全一致・ワイルドカード・IP・CIDR の4種類をサポートする。

### 作成/変更するファイル

| 操作 | ファイルパス | 説明 |
|------|-------------|------|
| 作成 | `internal/rule/matcher.go` | Matches / ValidateEntry 関数 |
| 作成 | `internal/rule/matcher_test.go` | マッチング・バリデーションの網羅的テスト |

---

## 技術的コンテキスト

### 使用技術

- 言語: Go 1.26
- 標準ライブラリ: `net`、`strings`

### 参照すべきファイル

- `@internal/rule/store.go` — `Entry` 型定義を参照

### 関連する設計書

- `@docs/sdd/design/components/matcher.md` — マッチングロジック詳細・テストケース一覧

### 関連する要件

- `@docs/sdd/requirements/stories/US-002.md` — ホワイトリスト制御・マッチングルール

---

## 受入基準

- [ ] `internal/rule/matcher.go` が作成されている
- [ ] 完全一致: `api.anthropic.com:443` → `{api.anthropic.com, 443}` にマッチ
- [ ] 完全一致: ポート違い → 不一致
- [ ] ワイルドカード: `github.com:443` → `{*.github.com, 443}` にマッチ
- [ ] ワイルドカード: `api.github.com:443` → `{*.github.com, 443}` にマッチ
- [ ] ワイルドカード: `evil.api.github.com:443` → `{*.github.com, 443}` に**不一致**
- [ ] IP 完全一致: `140.82.112.3:443` → `{140.82.112.3, 443}` にマッチ
- [ ] CIDR: `140.82.112.3:443` → `{140.82.112.0/20, 443}` にマッチ
- [ ] CIDR: 範囲外 IP → 不一致
- [ ] ポート 0: 全ポート許可
- [ ] 大文字入力: `API.ANTHROPIC.COM` → `{api.anthropic.com, 443}` にマッチ（正規化）
- [ ] 末尾ドット: `api.anthropic.com.:443` → マッチ（正規化）
- [ ] バリデーション: `*.*.example.com` → エラー
- [ ] バリデーション: `*.`（空 apex）→ エラー
- [ ] バリデーション: 空ホスト → エラー
- [ ] バリデーション: `https://github.com`（スキーム付き）→ エラー
- [ ] バリデーション: `api..github.com`（連続ドット）→ エラー
- [ ] バリデーション: port=99999 → エラー
- [ ] `go test ./internal/rule/...` が全パスする

---

## 実装手順

### ステップ 1: テストを先に作成（TDD）

`internal/rule/matcher_test.go` を作成し、以下を実装する:

```go
package rule_test

import "testing"
import "github.com/claudework/network-filter-proxy/internal/rule"

func TestMatches_ExactDomain(t *testing.T) { ... }
func TestMatches_ExactDomain_PortMismatch(t *testing.T) { ... }
func TestMatches_Wildcard_Apex(t *testing.T) { ... }
func TestMatches_Wildcard_Subdomain(t *testing.T) { ... }
func TestMatches_Wildcard_MultiLevel_NoMatch(t *testing.T) { ... }
func TestMatches_IPExact(t *testing.T) { ... }
func TestMatches_CIDR_Match(t *testing.T) { ... }
func TestMatches_CIDR_NoMatch(t *testing.T) { ... }
func TestMatches_PortZero_AllowsAny(t *testing.T) { ... }
func TestMatches_NormalizeUppercase(t *testing.T) { ... }
func TestMatches_NormalizeTrailingDot(t *testing.T) { ... }

func TestValidateEntry_EmptyHost(t *testing.T) { ... }
func TestValidateEntry_MultiLevelWildcard(t *testing.T) { ... }
func TestValidateEntry_EmptyWildcardApex(t *testing.T) { ... }  // "*." → エラー
func TestValidateEntry_SchemeInHost(t *testing.T) { ... }       // "https://github.com" → エラー
func TestValidateEntry_ConsecutiveDots(t *testing.T) { ... }    // "api..github.com" → エラー
func TestValidateEntry_InvalidPort(t *testing.T) { ... }
func TestValidateEntry_ValidCIDR(t *testing.T) { ... }
func TestValidateEntry_InvalidCIDR(t *testing.T) { ... }
```

テストを実行してコンパイルエラーを確認:
```bash
go test ./internal/rule/...
```

コミット: `test: Add Matcher unit tests`

### ステップ 2: matcher.go を実装

```go
package rule

import (
    "fmt"
    "net"
    "strings"
)

// Matches はエントリが宛先ホスト・ポートにマッチするか判定する
func Matches(entry Entry, host string, port int) bool {
    // 1. ポートチェック
    if entry.Port != 0 && entry.Port != port {
        return false
    }
    // 2. ホスト正規化
    host = strings.ToLower(strings.TrimSuffix(host, "."))
    entryHost := strings.ToLower(strings.TrimSuffix(entry.Host, "."))
    // 3a. CIDR 判定
    if strings.Contains(entryHost, "/") {
        _, ipNet, err := net.ParseCIDR(entryHost)
        if err != nil {
            return false
        }
        ip := net.ParseIP(host)
        return ip != nil && ipNet.Contains(ip)
    }
    // 3b. IP 完全一致
    if net.ParseIP(entryHost) != nil {
        return host == entryHost
    }
    // 3c. ワイルドカード（1階層サブドメインのみ許可）
    if strings.HasPrefix(entryHost, "*.") {
        apex := entryHost[2:] // "example.com"
        // apex 完全一致（例: "github.com" が "*.github.com" にマッチ）
        if host == apex {
            return true
        }
        // 直下1階層のサブドメインのみ許可
        // strings.HasSuffix だけでは "evil.api.github.com" が "*.github.com" にマッチしてしまうため、
        // ドット数で多段サブドメインを排除する
        suffix := "." + apex // ".example.com"
        return strings.HasSuffix(host, suffix) &&
            strings.Count(host, ".") == strings.Count(apex, ".")+1
    }
    // 3d. 完全一致
    return host == entryHost
}

// ValidateEntry はルールエントリのバリデーションを行う
func ValidateEntry(entry Entry) error {
    host := strings.TrimSpace(entry.Host)
    if host == "" {
        return fmt.Errorf("host is required")
    }
    if entry.Port < 0 || entry.Port > 65535 {
        return fmt.Errorf("port must be between 0 and 65535, got %d", entry.Port)
    }
    // スキーム付き URL を拒否（"://" を含む）
    if strings.Contains(host, "://") {
        return fmt.Errorf("invalid host: scheme not allowed: %s", host)
    }
    // 内部ホワイトスペースを拒否（TrimSpace では除去されない）
    if strings.ContainsAny(host, " \t") {
        return fmt.Errorf("invalid host: contains whitespace: %s", host)
    }
    // ワイルドカードバリデーション
    if strings.Contains(host, "*") {
        wildcardCount := strings.Count(host, "*")
        if wildcardCount > 1 || !strings.HasPrefix(host, "*.") {
            return fmt.Errorf("invalid wildcard pattern: %s (only *.example.com form is allowed)", host)
        }
        // apex が空（"*." のみ）の場合はエラー
        if len(host) <= 2 {
            return fmt.Errorf("invalid wildcard pattern: %s (apex domain is empty)", host)
        }
        // apex 部分もホスト名バリデーションを適用（"*.api..github.com" 等を拒否）
        apex := host[2:]
        if err := validateHostname(apex); err != nil {
            return fmt.Errorf("invalid wildcard apex %q: %w", apex, err)
        }
        return nil
    }
    // CIDR バリデーション
    if strings.Contains(host, "/") {
        if _, _, err := net.ParseCIDR(host); err != nil {
            return fmt.Errorf("invalid CIDR: %s", host)
        }
        return nil
    }
    // IP アドレス（完全一致）
    if net.ParseIP(host) != nil {
        return nil
    }
    // 完全一致ホスト名バリデーション
    return validateHostname(host)
}

// validateHostname は完全一致ホスト名のバリデーションを行う（連続ドット・ラベルのダッシュルール等）
func validateHostname(host string) error {
    if strings.Contains(host, "..") {
        return fmt.Errorf("invalid host: consecutive dots: %s", host)
    }
    if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
        return fmt.Errorf("invalid host: leading or trailing dot: %s", host)
    }
    // 各ラベルの先頭・末尾ダッシュを拒否（RFC 952 / RFC 1123）
    for _, label := range strings.Split(host, ".") {
        if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
            return fmt.Errorf("invalid host: label starts or ends with '-': %s", host)
        }
    }
    return nil
}
```

### ステップ 3: テストを実行して確認

```bash
go test ./internal/rule/...
```

### ステップ 4: コミット

```text
feat: Implement Matcher with wildcard/CIDR support
```

---

## 注意事項

- `*.github.com` は `github.com`（apex）と `api.github.com`（直下1階層のサブドメイン）の**両方**にマッチする
- `evil.api.github.com` のような多段サブドメインは `*.github.com` に**マッチしない**
- `strings.HasSuffix(host, suffix)` のみでの判定は `evil.api.github.com` が `*.github.com` にマッチしてしまうバグがあるため、ドット数による多段判定を必ず組み合わせること
- ホスト名は `strings.ToLower` + 末尾ドット除去で正規化してからマッチングする

---

## 基本情報（メタデータ）

| 項目 | 値 |
|------|-----|
| **タスクID** | TASK-003 |
| **ステータス** | `DONE` |
| **推定工数** | 30分 |
| **依存関係** | [TASK-002](TASK-002.md) @TASK-002.md（同フェーズ・並列実行可） |
| **対応要件** | REQ-002-001〜010, REQ-004-011〜013 |
| **対応設計** | components/matcher.md |

---

## 情報の明確性チェック

### 明示された情報

- [x] `*.github.com` は apex（`github.com`）にもマッチする
- [x] `*.*.example.com` は無効パターン
- [x] ポート 0 は全ポート許可
- [x] ホスト名の正規化（大文字・末尾ドット）でバイパス防止
