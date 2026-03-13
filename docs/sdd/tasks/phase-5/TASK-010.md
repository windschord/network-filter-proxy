# TASK-010: Dockerfile に HEALTHCHECK ディレクティブ追加 + ドキュメント更新

## 概要

Dockerfile に `HEALTHCHECK` ディレクティブを追加し、CLAUDE.md の環境変数テーブルを更新する。

## 対応する要件・設計

- 要件: [US-009](../../requirements/stories/US-009.md) @../../requirements/stories/US-009.md (REQ-009-006〜009)
- 要件: [US-008](../../requirements/stories/US-008.md) @../../requirements/stories/US-008.md (REQ-008-004)
- 設計: [index.md Docker ビルド](../../design/index.md) @../../design/index.md

## 情報の明確性

| 分類 | 内容 |
|------|------|
| 明示された情報 | interval=15s, timeout=5s, retries=3, CMD=["/filter-proxy", "healthcheck"] |
| 不明/要確認の情報 | なし |

## 対象ファイル

- `Dockerfile` -- HEALTHCHECK ディレクティブ追加
- `CLAUDE.md` -- 環境変数テーブルに `API_BIND_ADDR` 追加

## 技術的文脈

- ベースイメージ: `gcr.io/distroless/static:nonroot`
- HEALTHCHECK は ENTRYPOINT の前に配置
- 参照すべき既存コード: `Dockerfile`（現在の内容）

## 実装手順

### 1. Dockerfile 更新

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /filter-proxy ./cmd/filter-proxy

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /filter-proxy /filter-proxy
# Port 3128: proxy listener (all interfaces)
# Port 8080: management API (configurable via API_BIND_ADDR, default 127.0.0.1)
EXPOSE 3128 8080
HEALTHCHECK --interval=15s --timeout=5s --retries=3 \
  CMD ["/filter-proxy", "healthcheck"]
ENTRYPOINT ["/filter-proxy"]
```

### 2. CLAUDE.md 更新

環境変数テーブルに `API_BIND_ADDR` を追加:

```markdown
| `API_BIND_ADDR` | `127.0.0.1` | Management API バインドアドレス（不正値は `127.0.0.1` にフォールバック） |
```

### 3. コミット

## 受入基準

- [ ] Dockerfile に `HEALTHCHECK --interval=15s --timeout=5s --retries=3 CMD ["/filter-proxy", "healthcheck"]` が含まれる
- [ ] HEALTHCHECK が ENTRYPOINT の前に記述されている
- [ ] CLAUDE.md の環境変数テーブルに `API_BIND_ADDR` が追加されている
- [ ] `docker build .` が成功する（CI がある場合）

## 依存関係

TASK-009（healthcheck サブコマンドが実装済みであること）

## 推定工数

10分（AIエージェント作業時間）

## ステータス

`DONE`
