# API: Management API

## 概要

**ベースパス**: `/api/v1`
**目的**: ホワイトリストルールセットの動的な CRUD 操作とヘルスチェックを提供する

## 明示された情報

- デフォルト bind アドレス: `127.0.0.1:8080`
- 認証なし（Management API はコントロールプレーンのみ到達可能なネットワークに分離することを前提とする。認証が必要な運用形態では mTLS またはスコープ付きトークンによる認証/認可の追加を検討すること）
- レート制限なし
- Content-Type: `application/json`

---

## エンドポイント一覧

| メソッド | パス | 説明 | 成功コード |
|---------|------|------|-----------|
| GET | `/api/v1/health` | ヘルスチェック | 200 |
| GET | `/api/v1/rules` | 全ルール一覧取得 | 200 |
| PUT | `/api/v1/rules/{sourceIP}` | 指定 IP のルールセット全置換 | 200 |
| DELETE | `/api/v1/rules/{sourceIP}` | 指定 IP のルールセット削除 | 204 |
| DELETE | `/api/v1/rules` | 全ルール削除 | 204 |

---

## GET /api/v1/health

**説明**: プロキシと API の稼働状態を返す

### レスポンス

**200 OK**:
```json
{
  "status": "ok",
  "uptime_seconds": 3600,
  "active_connections": 5,
  "rule_count": 3
}
```

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `status` | string | 常に `"ok"` |
| `uptime_seconds` | int64 | プロセス起動からの経過秒数 |
| `active_connections` | int64 | 現在のアクティブ接続数 |
| `rule_count` | int | 登録済みルールセット数 |

---

## GET /api/v1/rules

**説明**: 登録されている全ルールセットを返す

### レスポンス

**200 OK**:
```json
{
  "rules": {
    "172.20.0.3": {
      "entries": [
        { "host": "api.anthropic.com", "port": 443 },
        { "host": "*.npmjs.org", "port": 443 }
      ]
    },
    "172.20.0.4": {
      "entries": [
        { "host": "api.anthropic.com", "port": 443 },
        { "host": "*.github.com", "port": 443 }
      ]
    }
  }
}
```

ルールが0件の場合:
```json
{ "rules": {} }
```

---

## PUT /api/v1/rules/{sourceIP}

**説明**: 指定した送信元 IP のルールセットを全置換する。存在しない場合は新規作成。

### パスパラメータ

| パラメータ | 型 | 説明 |
|-----------|-----|------|
| `sourceIP` | string | 送信元 IPv4 アドレス（例: `172.20.0.3`） |

### リクエスト

**Content-Type**: `application/json`

```json
{
  "entries": [
    { "host": "api.anthropic.com", "port": 443 },
    { "host": "*.npmjs.org", "port": 443 },
    { "host": "*.github.com" }
  ]
}
```

### バリデーションルール

| フィールド | ルール |
|-----------|--------|
| `entries` | 必須、配列（空配列も許可） |
| `entries[].host` | 必須、空文字不可、有効なドメイン / ワイルドカード / IP / CIDR |
| `entries[].host`（ワイルドカード） | `*` は先頭 1 セグメントのみ。`*.*.example.com` は不可 |
| `entries[].port` | 任意、1〜65535。省略または 0 で全ポート許可 |

### レスポンス

**200 OK**:
```json
{
  "source_ip": "172.20.0.3",
  "entries": [
    { "host": "api.anthropic.com", "port": 443 },
    { "host": "*.npmjs.org", "port": 443 },
    { "host": "*.github.com" }
  ],
  "updated_at": "2026-03-09T10:30:00Z"
}
```

**400 Bad Request**（バリデーションエラー）:
```json
{
  "error": "validation_error",
  "message": "invalid host pattern: **.example.com",
  "details": [
    { "field": "entries[0].host", "message": "invalid wildcard pattern" }
  ]
}
```

---

## DELETE /api/v1/rules/{sourceIP}

**説明**: 指定した送信元 IP のルールセットを削除する。削除後、その IP からの全通信は拒否される。

### パスパラメータ

| パラメータ | 型 | 説明 |
|-----------|-----|------|
| `sourceIP` | string | 送信元 IPv4 アドレス |

### レスポンス

**204 No Content**: 削除成功（ボディなし）

**404 Not Found**:
```json
{
  "error": "not_found",
  "message": "no rules found for source IP: 172.20.0.99"
}
```

---

## DELETE /api/v1/rules

**説明**: 全てのルールセットを削除する。全送信元からの通信が拒否される。

### レスポンス

**204 No Content**（ボディなし）

---

## エラーコード一覧

| `error` 値 | HTTPステータス | 説明 |
|------------|---------------|------|
| `validation_error` | 400 | リクエストボディのバリデーション失敗 |
| `bad_request` | 400 | JSON パース失敗等 |
| `not_found` | 404 | 対象リソースが存在しない |

---

## セキュリティ

- 認証: なし（コントロールプレーンのみ到達可能なネットワークに分離することを必須要件とする。認証が必要な場合は mTLS またはスコープ付きトークンを検討すること）
- CORS: 設定なし
- レート制限: なし
- **バインドアドレス**: Management API サーバーは `127.0.0.1`（loopback）または専用 internal network インターフェースにのみ bind しなければならない（必須）。`0.0.0.0` への bind は禁止。デフォルトは `127.0.0.1`
- **ネットワーク分離**: Management API は外部ネットワークから直接到達不可能でなければならない。信頼されたネットワークセグメント（internal network）からのみアクセス可能とすること（REQ-004-014）

## 関連コンポーネント

- [api-handler](../components/api-handler.md) @../components/api-handler.md: リクエスト処理
- [rule-store](../components/rule-store.md) @../components/rule-store.md: データ管理

## 関連要件

- [US-004](../../requirements/stories/US-004.md) @../../requirements/stories/US-004.md: Management REST API
- [US-005](../../requirements/stories/US-005.md) @../../requirements/stories/US-005.md: ヘルスチェック
