# TASK-002: RuleStore 実装（メモリ内ルール CRUD）

> **サブエージェント実行指示**
> このドキュメントは、タスク実行エージェントがサブエージェントにそのまま渡すことを想定しています。
> 以下の内容に従って実装を完了してください。

---

## あなたのタスク

**RuleStore** を実装してください。送信元 IP をキーとしたルールセットをメモリ上で管理し、スレッドセーフな CRUD を提供します。

### 実装の目標

`sync.RWMutex` で保護されたインメモリマップを用い、送信元 IP ごとのルールセット（エントリリスト）を管理する。`go test -race` で競合が検出されないこと。

### 作成/変更するファイル

| 操作 | ファイルパス | 説明 |
|------|-------------|------|
| 作成 | `internal/rule/store.go` | RuleStore・Entry・RuleSet 型定義と CRUD |
| 作成 | `internal/rule/store_test.go` | RuleStore のユニットテスト（race 含む） |

---

## 技術的コンテキスト

### 使用技術

- 言語: Go 1.26
- 標準ライブラリ: `sync`、`time`
- テスト: `go test -race ./internal/rule/...`

### 関連する設計書

- `@docs/sdd/design/components/rule-store.md` — RuleStore の型定義・メソッド仕様

### 関連する要件

- `@docs/sdd/requirements/stories/US-003.md` — 送信元 IP ベースのルール識別

---

## 受入基準

- [ ] `internal/rule/store.go` が作成されている
- [ ] `Entry`、`RuleSet`、`Store` 型が定義されている
- [ ] `NewStore()`、`Get()`、`Set()`、`Delete()`、`DeleteAll()`、`All()`、`Count()` が実装されている
- [ ] `Set → Get` でルールセットが取得できる
- [ ] `Set → Delete → Get` で `(nil, false)` が返る
- [ ] `DeleteAll` で全ルールが消える
- [ ] `All()` が返すマップを変更しても元ストアに影響しない（ディープコピー）
- [ ] `go test -race ./internal/rule/...` が全パスする（競合なし）

---

## 実装手順

### ステップ 1: テストを先に作成（TDD）

`internal/rule/store_test.go` を作成し、以下のテストケースを実装する:

```go
package rule_test

import (
    "testing"
    "sync"
    "github.com/claudework/network-filter-proxy/internal/rule"
)

func TestStore_SetAndGet(t *testing.T) { ... }
func TestStore_GetNotFound(t *testing.T) { ... }
func TestStore_Delete(t *testing.T) { ... }
func TestStore_DeleteNotFound(t *testing.T) { ... }
func TestStore_DeleteAll(t *testing.T) { ... }
func TestStore_AllIsDeepCopy(t *testing.T) { ... }
func TestStore_Count(t *testing.T) { ... }
func TestStore_ConcurrentAccess(t *testing.T) {
    // 複数 goroutine から同時に Set/Get を呼び出す
    // go test -race で競合検出
}
```

テストを実行してコンパイルエラーを確認:
```bash
go test ./internal/rule/...
```

コミット: `test: Add RuleStore unit tests`

### ステップ 2: store.go を実装

```go
package rule

import (
    "sync"
    "time"
)

type Entry struct {
    Host string
    Port int // 0 = 全ポート許可
}

type RuleSet struct {
    Entries   []Entry
    UpdatedAt time.Time
}

type Store struct {
    mu    sync.RWMutex
    rules map[string]*RuleSet
}

func NewStore() *Store {
    return &Store{rules: make(map[string]*RuleSet)}
}

func (s *Store) Get(sourceIP string) (*RuleSet, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    rs, ok := s.rules[sourceIP]
    if !ok {
        return nil, false
    }
    // 呼び出し元が Entries を変更しても内部状態に影響しないよう deep copy を返す
    entries := make([]Entry, len(rs.Entries))
    copy(entries, rs.Entries)
    return &RuleSet{Entries: entries, UpdatedAt: rs.UpdatedAt}, true
}

func (s *Store) Set(sourceIP string, entries []Entry) {
    s.mu.Lock()
    defer s.mu.Unlock()
    copied := make([]Entry, len(entries))
    copy(copied, entries)
    s.rules[sourceIP] = &RuleSet{
        Entries:   copied,
        UpdatedAt: time.Now().UTC(),
    }
}

func (s *Store) Delete(sourceIP string) bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    _, ok := s.rules[sourceIP]
    if ok {
        delete(s.rules, sourceIP)
    }
    return ok
}

func (s *Store) DeleteAll() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.rules = make(map[string]*RuleSet)
}

func (s *Store) All() map[string]*RuleSet {
    s.mu.RLock()
    defer s.mu.RUnlock()
    snap := make(map[string]*RuleSet, len(s.rules))
    for k, v := range s.rules {
        entries := make([]Entry, len(v.Entries))
        copy(entries, v.Entries)
        snap[k] = &RuleSet{Entries: entries, UpdatedAt: v.UpdatedAt}
    }
    return snap
}

func (s *Store) Count() int {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return len(s.rules)
}
```

### ステップ 3: テストを実行して確認

```bash
go test ./internal/rule/...
go test -race ./internal/rule/...
```

### ステップ 4: コミット

```text
feat: Implement RuleStore with thread-safe CRUD
```

---

## 注意事項

- `Set()` 時は `entries` スライスをコピーする（呼び出し元のスライスへの依存を防ぐ）
- `All()` の戻り値はスナップショットコピー。元マップへの参照を漏らさない
- `Get()` は `RLock`（読み取りロック）を使用し、書き込みと独立して並行実行可能にする

---

## 基本情報（メタデータ）

| 項目 | 値 |
|------|-----|
| **タスクID** | TASK-002 |
| **ステータス** | `DONE` |
| **推定工数** | 25分 |
| **依存関係** | [TASK-001](../phase-1/TASK-001.md) @../phase-1/TASK-001.md |
| **対応要件** | REQ-003-001〜004 |
| **対応設計** | components/rule-store.md |

---

## 情報の明確性チェック

### 明示された情報

- [x] `sync.RWMutex` でスレッドセーフにする
- [x] メモリのみ保持（永続化なし）
- [x] `go test -race` で競合検出すること
- [x] `All()` はスナップショットコピーを返す
