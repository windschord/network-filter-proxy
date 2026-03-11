# 要件定義

## 概要

ClaudeWork が管理する Docker 環境内の ClaudeCode コンテナに対し、ホワイトリスト方式でアウトバウンド通信を制限するフォワードプロキシ（Network Filter Proxy）の要件定義。

Go + `elazarl/goproxy` で実装し、独立したリポジトリで管理する。

### システム構成概要

```text
┌─ Docker internal network (--internal) ────────────────┐
│  [Claude Container A]    [Claude Container B]          │
│    HTTP_PROXY=proxy:3128   HTTP_PROXY=proxy:3128       │
│         │                       │                      │
│         └───────┐  ┌────────────┘                      │
│                 ▼  ▼                                   │
│  [Filter Proxy Container]                              │
│    :3128  - forward proxy (CONNECT 対応)               │
│    127.0.0.1:8080  - management API (localhost only)   │
└────────┼───────────────────────────────────────────────┘
         │ (external network にも接続)
         ▼
     Internet (ホワイトリスト宛先のみ)
```

## ユーザーストーリー一覧

| ID | タイトル | 優先度 | ステータス | 詳細リンク |
|----|---------|--------|-----------|------------|
| US-001 | HTTPS フォワードプロキシ | 高 | 承認済 | [詳細](stories/US-001.md) @stories/US-001.md |
| US-002 | ドメインベースのホワイトリスト制御 | 高 | 承認済 | [詳細](stories/US-002.md) @stories/US-002.md |
| US-003 | 送信元 IP ベースの環境識別とデフォルト拒否 | 高 | 承認済 | [詳細](stories/US-003.md) @stories/US-003.md |
| US-004 | Management REST API によるルール動的管理 | 高 | 承認済 | [詳細](stories/US-004.md) @stories/US-004.md |
| US-005 | ヘルスチェック | 中 | 承認済 | [詳細](stories/US-005.md) @stories/US-005.md |
| US-006 | 構造化ログ出力 | 中 | 承認済 | [詳細](stories/US-006.md) @stories/US-006.md |
| US-007 | Graceful Shutdown | 高 | 承認済 | [詳細](stories/US-007.md) @stories/US-007.md |

## 機能要件サマリ

| 要件ID | 概要 | 関連ストーリー | ステータス |
|--------|------|---------------|-----------|
| REQ-001-001〜005 | HTTP CONNECT / HTTP フォワードプロキシ | US-001 | 定義済 |
| REQ-002-001〜010 | ドメイン・IP・CIDR・ワイルドカードマッチング | US-002 | 定義済 |
| REQ-003-001〜004 | 送信元 IP によるルールセット適用とデフォルト拒否 | US-003 | 定義済 |
| REQ-004-001〜014 | Management REST API（CRUD・バリデーション） | US-004 | 定義済 |
| REQ-005-001〜003 | ヘルスチェックエンドポイント | US-005 | 定義済 |
| REQ-006-001〜009 | 構造化 JSON ログ（接続・ルール変更操作・フォールバック） | US-006 | 定義済 |
| REQ-007-001〜005 | SIGTERM による Graceful Shutdown | US-007 | 定義済 |

## 非機能要件一覧

| カテゴリ | 詳細リンク | 要件数 |
|----------|------------|--------|
| 性能要件 | [詳細](nfr/performance.md) @nfr/performance.md | 3件 |
| セキュリティ要件 | [詳細](nfr/security.md) @nfr/security.md | 5件 |
| 保守性要件 | [詳細](nfr/maintainability.md) @nfr/maintainability.md | 4件 |

## 依存関係

- **外部ライブラリ**: `elazarl/goproxy`（Go フォワードプロキシ実装）
- **ベースイメージ**: `gcr.io/distroless/static` または `scratch`（Go static binary）
- **連携システム**: ClaudeWork（コンテナ起動・管理・ルール登録を行う上位システム）

## スコープ外

- TLS 終端・MITM による通信内容の検査
- Management API の認証・認可機能（`127.0.0.1` バインド固定により外部からのアクセスを遮断しているため不要）
- ルールの永続化（再起動時はルールが消える。ClaudeWork が再登録する）
- HTTP/2・gRPC プロキシ対応（HTTP/1.1 CONNECT のみ）
- 複数インスタンスでの分散運用・ルール同期
