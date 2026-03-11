# TASK-007: Dockerfile・CI 設定

> **サブエージェント実行指示**
> このドキュメントは、タスク実行エージェントがサブエージェントにそのまま渡すことを想定しています。
> 以下の内容に従って実装を完了してください。

---

## あなたのタスク

**Dockerfile** と **GitHub Actions CI** を実装してください。`gcr.io/distroless/static` ベースの Docker イメージとテスト・lint の CI パイプラインを構築します。

### 実装の目標

マルチステージビルドで静的バイナリを生成し、`gcr.io/distroless/static` ベースの軽量イメージを作成する。GitHub Actions で `go test -race` と `golangci-lint` を実行する CI を構築する。

### 作成/変更するファイル

| 操作 | ファイルパス | 説明 |
|------|-------------|------|
| 作成 | `Dockerfile` | マルチステージビルド定義 |
| 作成 | `.github/workflows/ci.yml` | CI パイプライン |
| 作成 | `.golangci.yml` | golangci-lint 設定 |

---

## 技術的コンテキスト

### 使用技術

- Docker: マルチステージビルド
- ベースイメージ: `golang:1.26-alpine`（ビルド）/ `gcr.io/distroless/static`（実行）
- CI: GitHub Actions
- Linter: `golangci-lint`

### 関連する設計書

- `@docs/sdd/design/index.md` — CI/CD 設計・品質ゲート

### 関連する要件

- `@docs/sdd/requirements/nfr/maintainability.md` — テストカバレッジ・静的解析・イメージサイズ

---

## 受入基準

- [ ] `Dockerfile` が作成されている
- [ ] `docker build -t filter-proxy .` が成功する
- [ ] イメージサイズが 30MB 以下
- [ ] `docker run --rm filter-proxy` でバイナリが起動する
- [ ] `.github/workflows/ci.yml` が作成されている
- [ ] CI で `go test -race ./...` が実行される
- [ ] CI で `golangci-lint run` が実行される
- [ ] `.golangci.yml` が作成されている

---

## 実装手順

### ステップ 1: Dockerfile を作成

**注意**: `gcr.io/distroless/static:nonroot` には `curl` が存在しないため `HEALTHCHECK` 命令は使用しない。ヘルスチェックは Docker Compose / Kubernetes の外部 HTTP プローブに委ねる（US-005）。

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /filter-proxy ./cmd/filter-proxy

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /filter-proxy /filter-proxy
EXPOSE 3128 8080
ENTRYPOINT ["/filter-proxy"]
```

### ステップ 2: .golangci.yml を作成

```yaml
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - cyclop

linters-settings:
  cyclop:
    max-complexity: 10

run:
  go: "1.26"
  timeout: 5m
```

### ステップ 3: CI を作成

`.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
      - name: Run tests
        run: go test -race -coverprofile=coverage.out ./...
      - name: Check coverage
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%","",$3); print $3}')
          awk -v c="$COVERAGE" 'BEGIN { exit (c >= 80 ? 0 : 1) }'
      - name: golangci-lint
        uses: golangci/golangci-lint-action@971cd0a9f32b42f9b5a6e2979416e0e2e1573a7f  # v6.5.1
        with:
          version: v1.64.8

  build:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v4
      - name: Build Docker image
        run: docker build -t filter-proxy .
      - name: Check image size
        run: |
          SIZE=$(docker image inspect filter-proxy --format='{{.Size}}')
          echo "Image size: $SIZE bytes"
          # 30MB = 31457280 bytes
          [ "$SIZE" -lt "31457280" ] && echo "OK" || (echo "Image too large" && exit 1)
```

### ステップ 4: コミット

```text
feat: Add Dockerfile and GitHub Actions CI
```

---

## 注意事項

- `CGO_ENABLED=0` で静的バイナリをビルドする（distroless に必要）
- `-ldflags="-s -w"` でバイナリサイズを削減する
- `gcr.io/distroless/static:nonroot` を使い、root 以外のユーザーで実行する

---

## 基本情報（メタデータ）

| 項目 | 値 |
|------|-----|
| **タスクID** | TASK-007 |
| **ステータス** | `DONE` |
| **推定工数** | 25分 |
| **依存関係** | [TASK-006](TASK-006.md) @TASK-006.md |
| **対応要件** | NFR-MNT-001, NFR-MNT-002, NFR-MNT-003 |
| **対応設計** | design/index.md（CI/CD 設計） |

---

## 情報の明確性チェック

### 明示された情報

- [x] ベースイメージ: `gcr.io/distroless/static`
- [x] `CGO_ENABLED=0` で静的バイナリ
- [x] イメージサイズ 30MB 以下
- [x] テストカバレッジ 80% 以上
- [x] `golangci-lint` でエラー 0
