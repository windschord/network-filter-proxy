# Matcher コンポーネント

## 概要

**目的**: ルールエントリのホストパターンと宛先ホスト・ポートのマッチング判定を行う

**責務**:
- 完全一致ドメインマッチング
- ワイルドカードパターン（`*.example.com`）マッチング
- IP アドレス完全一致マッチング
- CIDR 範囲マッチング
- ポート番号マッチング（0 = 全許可）
- ホスト名の正規化（大文字小文字統一・末尾ドット除去）

## 明示された情報

- `*.github.com` は `github.com` 自体と直下1階層のサブドメイン（`api.github.com` 等）にマッチ。`evil.api.github.com` のような多段サブドメインはマッチしない
- `*.*.example.com` は無効パターン
- CIDR: `140.82.112.0/20` など
- ポート 0 の場合は全ポート許可
- ホスト名正規化でバイパス防止（NFR-SEC-004）

---

## インターフェース（Go）

### パッケージ: `internal/rule`

```go
// Matches はエントリが宛先ホスト・ポートにマッチするか判定する
// host: 宛先ホスト名または IP アドレス文字列
// port: 宛先ポート番号
func Matches(entry Entry, host string, port int) bool

// ValidateEntry はルールエントリのバリデーションを行う
// エラー内容を返す（nil = 有効）
func ValidateEntry(entry Entry) error
```

---

## マッチングロジック詳細

```text
Matches(entry, host, port):
  1. ポートチェック
     - entry.Port != 0 かつ entry.Port != port → false
  2. ホスト正規化
     - host = strings.ToLower(strings.TrimSuffix(host, "."))
     - entryHost = strings.ToLower(strings.TrimSuffix(entry.Host, "."))
  3. パターン判定
     a. CIDR 判定 (strings.Contains(entryHost, "/"))
        → net.ParseCIDR で CIDR パース
        → net.ParseIP(host) が CIDR 内なら true
     b. IP 判定 (net.ParseIP(entryHost) != nil)
        → host == entryHost なら true
     c. ワイルドカード判定 (strings.HasPrefix(entryHost, "*."))
        → apex = entryHost[2:]  // "example.com"
        → host == apex なら true（apex 完全一致）
        → host == "サブドメイン." + apex（直下1階層のみ）なら true
          具体的には: strings.HasSuffix(host, "."+apex) かつ
                     strings.Count(host, ".") == strings.Count(apex, ".")+1 なら true
        （`evil.api.example.com` のような多段サブドメインは不一致）
     d. 完全一致
        → host == entryHost なら true
  4. 上記に該当しない → false
```

### バリデーションロジック

```text
ValidateEntry(entry):
  - entry.Host が空 → エラー
  - entry.Port < 0 または > 65535 → エラー
  - ワイルドカードの場合:
    - "*" の個数が 1 より多い → エラー
    - "*" が先頭以外にある → エラー
    - "*.*.example.com" → エラー
  - CIDR の場合:
    - net.ParseCIDR で解析失敗 → エラー
```

---

## 依存関係

### 依存するコンポーネント
- Go 標準ライブラリ `net`（IP / CIDR 解析）

### 依存されるコンポーネント
- [proxy-handler](proxy-handler.md) @proxy-handler.md: 接続判定時に使用
- [api-handler](api-handler.md) @api-handler.md: ルール登録時のバリデーションに使用

---

## テスト観点

- [ ] 完全一致: `api.anthropic.com:443` → エントリ `{api.anthropic.com, 443}` にマッチ
- [ ] 完全一致: `api.anthropic.com:80` → エントリ `{api.anthropic.com, 443}` に不一致
- [ ] ワイルドカード: `github.com:443` → エントリ `{*.github.com, 443}` にマッチ
- [ ] ワイルドカード: `api.github.com:443` → エントリ `{*.github.com, 443}` にマッチ
- [ ] ワイルドカード: `evil.api.github.com:443` → エントリ `{*.github.com, 443}` に不一致
- [ ] IP 完全一致: `140.82.112.3:443` → エントリ `{140.82.112.3, 443}` にマッチ
- [ ] CIDR: `140.82.112.3:443` → エントリ `{140.82.112.0/20, 443}` にマッチ
- [ ] CIDR: `192.168.1.1:443` → エントリ `{140.82.112.0/20, 443}` に不一致
- [ ] ポート省略: `api.anthropic.com:80` → エントリ `{api.anthropic.com, 0}` にマッチ
- [ ] 正規化: `API.ANTHROPIC.COM:443` → エントリ `{api.anthropic.com, 443}` にマッチ
- [ ] 正規化: `api.anthropic.com.:443`（末尾ドット）→ エントリ `{api.anthropic.com, 443}` にマッチ
- [ ] 正規化: エントリ `{api.anthropic.com., 443}` → `api.anthropic.com:443` にマッチ（entry.Host 側の末尾ドット正規化）
- [ ] バリデーション: `*.*.example.com` → エラー
- [ ] バリデーション: 空ホスト → エラー
- [ ] バリデーション: port=99999 → エラー

## 関連要件

- [US-002](../../requirements/stories/US-002.md) @../../requirements/stories/US-002.md: ホワイトリスト制御・マッチングルール
- [NFR-SEC-004](../../requirements/nfr/security.md) @../../requirements/nfr/security.md: ルールバイパス防止
