# CLAUDE.md — Network Filter Proxy

## プロジェクト概要

Go 製フォワードプロキシ。送信元 IP ベースのホワイトリストで HTTPS CONNECT / 通常 HTTP をフィルタリングする。

## 技術スタック

| 項目 | 値 |
|------|-----|
| 言語 | Go 1.26 |
| プロキシライブラリ | `github.com/elazarl/goproxy` |
| ログ | `log/slog`（JSON / text、`timestamp` フィールド） |
| ルール保持 | インメモリ（`sync.RWMutex` + `map`） |
| Management API | `{API_BIND_ADDR}:{API_PORT}`（デフォルト `127.0.0.1:8080`） |
| Proxy ポート | `:3128` |

## ディレクトリ構成

```text
cmd/filter-proxy/   # エントリポイント (main.go)
internal/
  config/           # 環境変数読み込み
  logger/           # slog ファクトリ（ReplaceAttr で time→timestamp）
  rule/             # RuleStore (CRUD, sync.RWMutex) / Matcher
  proxy/            # ProxyHandler (goproxy ラッパー)
  api/              # APIHandler (Management REST API)
docs/sdd/           # SDD ドキュメント（要件・設計・タスク）
```

## 開発コマンド

```bash
# ドキュメント lint
npm run textlint

# PR レビューコメント一覧
make pr-check

# PR レビューコメントを Claude で自動修正（通常モード・都度承認あり）
make pr-fix

# PR レビューコメントを Claude で自動修正（隔離環境専用・権限承認スキップ）
make pr-fix-unsafe
```

## PR レビュー対応ワークフロー

### 基本的な流れ

1. `make pr-check` でコメント一覧を確認
2. `make pr-fix` で Claude が自動修正・コミット・プッシュ
3. CodeRabbit / Copilot の再レビューを待機
4. コメントがなくなるまで繰り返す

### 権限承認を省略する方法（隔離環境専用）

`make pr-fix-unsafe` は `claude --dangerously-skip-permissions` で実行するため、
`gh api` / `git` / `npm` などの都度承認が不要。
**注意**: このコマンドは隔離済みの検証環境（worktree 等）でのみ使用してください。
通常の開発フローでは `make pr-fix`（通常モード）を使用してください。

### 手動で Claude に依頼する場合

```text
/fix-pr-comments
```

または直接プロンプト:

```text
PR #<番号> のレビューコメントをすべて修正してください。
指摘がなくなるまで繰り返してください。
```

## 重要な設計判断

- **API バインドアドレス**: Management API はデフォルト `127.0.0.1`。`API_BIND_ADDR` で変更可能（`0.0.0.0` は Docker Compose 環境用。本番環境では適切なネットワーク分離を確保すること）
- **HEALTHCHECK**: distroless には curl がないため Go バイナリの `healthcheck` サブコマンドで実現
- **activeConn**: `trackingConn` ラッパーで `Close()` 時に `Add(-1)` するパターンを使用
- **ワイルドカード**: `*.example.com` は apex + 直下1階層のみマッチ（多段サブドメイン不可）
- **os.Exit**: `main()` では `os.Exit(run())` パターンを使い、defer が実行される構造にする

## 環境変数

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `PROXY_PORT` | `3128` | プロキシリッスンポート |
| `API_PORT` | `8080` | Management API ポート |
| `API_BIND_ADDR` | `127.0.0.1` | Management API バインドアドレス（不正値は `127.0.0.1` にフォールバック） |
| `LOG_FORMAT` | `json` | `json` / `text`（不正値は `json` にフォールバック） |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error`（不正値は `info` にフォールバック） |
| `SHUTDOWN_TIMEOUT` | `30` | Graceful shutdown 秒数（負値・不正値は 30 にフォールバック） |
