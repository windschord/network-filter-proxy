# タスク

> このドキュメントはAIエージェント（Claude Code等）が実装を行うことを前提としています。
> **不明な情報が1つでもある場合は、実装前に必ず確認を取ってください。**

## 情報の明確性チェック（全体）

### ユーザーから明示された情報

- [x] 言語: Go 1.24
- [x] モジュール名: `github.com/claudework/network-filter-proxy`
- [x] テスト: `go test -race ./...`（競合検出）
- [x] Linter: `golangci-lint`
- [x] カバレッジ目標: 80% 以上
- [x] TDD で進める（テスト → 実装の順）

### 不明/要確認の情報

なし（全情報が要件・設計に明示済み）

---

## 進捗サマリ

| フェーズ | 完了 | 進行中 | 未着手 | ブロック | 詳細リンク |
|---------|------|--------|--------|----------|-----------|
| Phase 1: プロジェクト基盤 | 0 | 0 | 1 | 0 | [詳細](phase-1/) @phase-1/ |
| Phase 2: コアロジック | 0 | 0 | 2 | 0 | [詳細](phase-2/) @phase-2/ |
| Phase 3: サーバー実装 | 0 | 0 | 2 | 0 | [詳細](phase-3/) @phase-3/ |
| Phase 4: 統合・Docker | 0 | 0 | 2 | 0 | [詳細](phase-4/) @phase-4/ |

---

## タスク一覧

### Phase 1: プロジェクト基盤
*推定期間: 20分（AIエージェント作業時間）*

| タスクID | タイトル | ステータス | 依存 | 見積 | 詳細リンク |
|----------|---------|-----------|------|------|-----------|
| TASK-001 | プロジェクト基盤構築（go.mod / config / logger） | TODO | - | 20min | [詳細](phase-1/TASK-001.md) @phase-1/TASK-001.md |

### Phase 2: コアロジック
*推定期間: 30〜35分（AIエージェント作業時間・並列実行可能）*

| タスクID | タイトル | ステータス | 依存 | 見積 | 詳細リンク |
|----------|---------|-----------|------|------|-----------|
| TASK-002 | RuleStore 実装（メモリ内ルール CRUD） | TODO | TASK-001 | 25min | [詳細](phase-2/TASK-002.md) @phase-2/TASK-002.md |
| TASK-003 | Matcher 実装（ホスト・ポートマッチング + バリデーション） | TODO | TASK-001 | 30min | [詳細](phase-2/TASK-003.md) @phase-2/TASK-003.md |

### Phase 3: サーバー実装
*推定期間: 45分（AIエージェント作業時間・並列実行可能）*

| タスクID | タイトル | ステータス | 依存 | 見積 | 詳細リンク |
|----------|---------|-----------|------|------|-----------|
| TASK-004 | ProxyHandler 実装（HTTP CONNECT フィルタリング） | TODO | TASK-002, TASK-003 | 40min | [詳細](phase-3/TASK-004.md) @phase-3/TASK-004.md |
| TASK-005 | APIHandler 実装（Management REST API） | TODO | TASK-002, TASK-003 | 45min | [詳細](phase-3/TASK-005.md) @phase-3/TASK-005.md |

### Phase 4: 統合・Docker
*推定期間: 35〜55分（AIエージェント作業時間）*

| タスクID | タイトル | ステータス | 依存 | 見積 | 詳細リンク |
|----------|---------|-----------|------|------|-----------|
| TASK-006 | main.go 統合（DI・Graceful Shutdown） | TODO | TASK-004, TASK-005 | 30min | [詳細](phase-4/TASK-006.md) @phase-4/TASK-006.md |
| TASK-007 | Dockerfile・CI 設定 | TODO | TASK-006 | 25min | [詳細](phase-4/TASK-007.md) @phase-4/TASK-007.md |

---

## 並列実行グループ

### グループ 1（Phase 1 完了後、並列実行可能）

| タスク | 対象ファイル | 依存 |
|--------|-------------|------|
| TASK-002 | `internal/rule/store.go`, `store_test.go` | TASK-001 |
| TASK-003 | `internal/rule/matcher.go`, `matcher_test.go` | TASK-001 |

### グループ 2（Phase 2 完了後、並列実行可能）

| タスク | 対象ファイル | 依存 |
|--------|-------------|------|
| TASK-004 | `internal/proxy/handler.go`, `handler_test.go` | TASK-002, TASK-003 |
| TASK-005 | `internal/api/handler.go`, `handler_test.go` | TASK-002, TASK-003 |

---

## タスクステータスの凡例

- `TODO` - 未着手
- `IN_PROGRESS` - 作業中
- `BLOCKED` - 依存関係や問題によりブロック中
- `REVIEW` - レビュー待ち
- `DONE` - 完了

---

## 依存関係グラフ

```text
TASK-001 (基盤)
    ├── TASK-002 (RuleStore)   ┐
    └── TASK-003 (Matcher)    ┘ ← 並列実行可能
              ├── TASK-004 (ProxyHandler) ┐
              └── TASK-005 (APIHandler)   ┘ ← 並列実行可能
                        └── TASK-006 (main.go)
                                  └── TASK-007 (Dockerfile/CI)
```

---

## リスクと軽減策

| リスク | 影響度 | 発生確率 | 軽減策 |
|--------|--------|----------|--------|
| `elazarl/goproxy` の CONNECT 拒否レスポンス設定が複雑 | 中 | 中 | `ctx.Resp` に事前設定する方式を採用（設計に明記済み） |
| Go 1.22 ServeMux のパスパラメータ構文の非互換 | 低 | 低 | Go 1.24 使用のため問題なし |
| distroless イメージで CGO 依存バイナリが起動しない | 高 | 低 | `CGO_ENABLED=0` で静的ビルドを徹底 |

## 備考

- 全タスクで TDD（テスト先行）で進めること
- `go test -race ./...` を各タスク完了時に実行すること
- Phase 2 の TASK-002 と TASK-003 は同じパッケージ（`internal/rule`）に属するが、対象ファイルが異なるため並列実行可能
