# -------------------------------------------------------------------
# Network Filter Proxy — 開発補助タスク
# -------------------------------------------------------------------

# GitHub リポジトリを git remote から自動取得
GITHUB_REPO ?= $(shell git remote get-url origin 2>/dev/null \
	| sed 's|.*github\.com[:/]||' | sed 's|\.git$$||')

# 現在チェックアウト中のブランチに紐付く PR 番号を自動取得
PR_NUMBER ?= $(shell gh pr view --json number -q .number 2>/dev/null || echo "")

.PHONY: help pr-check pr-fix pr-fix-unsafe swagger

help: ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

# ------------------------------------------------------------------
# OpenAPI / Swagger
# ------------------------------------------------------------------

swagger: ## swag init で OpenAPI ドキュメントを再生成
	swag init -g cmd/filter-proxy/main.go -o docs/swagger --outputTypes yaml

# ------------------------------------------------------------------
# PR レビュー対応
# ------------------------------------------------------------------

pr-check: ## 現在の PR のインラインコメント＋レビューを一覧表示
	@[ -n "$(PR_NUMBER)" ] || (echo "ERROR: PR が見つかりません (gh pr view に失敗)"; exit 1)
	@echo "=== PR #$(PR_NUMBER) inline comments ($(GITHUB_REPO)) ==="
	@gh api repos/$(GITHUB_REPO)/pulls/$(PR_NUMBER)/comments --paginate --slurp \
	  | python3 -c "\
import json, sys; \
pages = json.load(sys.stdin); \
cs = [c for page in pages for c in page]; \
cs.sort(key=lambda c: c['created_at']); \
print(f'Total: {len(cs)} inline comments'); \
[print(f'[{c[\"id\"]}] {c[\"created_at\"]} {c[\"user\"][\"login\"]} - {c[\"path\"]}:{c.get(\"line\",\"?\")}\n  {c[\"body\"][:300]}\n') for c in cs]"
	@echo ""
	@echo "=== PR #$(PR_NUMBER) reviews ($(GITHUB_REPO)) ==="
	@gh api repos/$(GITHUB_REPO)/pulls/$(PR_NUMBER)/reviews --paginate \
	  | python3 -c "\
import json, sys; \
rs = json.load(sys.stdin); \
rs.sort(key=lambda r: r['submitted_at']); \
print(f'Total: {len(rs)} reviews'); \
[print(f'[{r[\"id\"]}] {r[\"submitted_at\"]} {r[\"user\"][\"login\"]} state={r[\"state\"]}\n  {r[\"body\"][:300]}\n') for r in rs if r[\"body\"]]"

pr-fix: ## PR のレビューコメントを Claude で自動修正（通常モード）
	@[ -n "$(PR_NUMBER)" ] || (echo "ERROR: PR が見つかりません (gh pr view に失敗)"; exit 1)
	@[ -z "$$(git status --porcelain)" ] || (echo "ERROR: 未コミット変更があります。先に commit か stash をしてください"; exit 1)
	@BRANCH=$$(git rev-parse --abbrev-ref HEAD) && \
	echo "=== Fixing PR #$(PR_NUMBER) on branch $$BRANCH ===" && \
	claude -p \
	  "PR #$(PR_NUMBER) (https://github.com/$(GITHUB_REPO)/pull/$(PR_NUMBER)) のレビューコメントをすべて修正してください。作業ブランチ: $$BRANCH。手順: 1) gh api repos/$(GITHUB_REPO)/pulls/$(PR_NUMBER)/comments --paginate --slurp でインラインコメントを取得 2) gh api repos/$(GITHUB_REPO)/pulls/$(PR_NUMBER)/reviews --paginate でレビュー全体コメントを取得 3) 未対応の指摘を特定して修正 4) npm run textlint で検証（Markdown lint がある場合） 5) 変更したファイルのみ stage して git commit -m 'fix: Address PR review comments' && git push origin $$BRANCH"

pr-fix-unsafe: ## PR のレビューコメントを Claude で自動修正（権限承認スキップ・隔離環境専用）
	@[ -n "$(PR_NUMBER)" ] || (echo "ERROR: PR が見つかりません (gh pr view に失敗)"; exit 1)
	@[ -z "$$(git status --porcelain)" ] || (echo "ERROR: 未コミット変更があります。先に commit か stash をしてください"; exit 1)
	@BRANCH=$$(git rev-parse --abbrev-ref HEAD) && \
	echo "=== Fixing PR #$(PR_NUMBER) on branch $$BRANCH (UNSAFE MODE) ===" && \
	claude --dangerously-skip-permissions -p \
	  "PR #$(PR_NUMBER) (https://github.com/$(GITHUB_REPO)/pull/$(PR_NUMBER)) のレビューコメントをすべて修正してください。作業ブランチ: $$BRANCH。手順: 1) gh api repos/$(GITHUB_REPO)/pulls/$(PR_NUMBER)/comments --paginate --slurp でインラインコメントを取得 2) gh api repos/$(GITHUB_REPO)/pulls/$(PR_NUMBER)/reviews --paginate でレビュー全体コメントを取得 3) 未対応の指摘を特定して修正 4) npm run textlint で検証（Markdown lint がある場合） 5) 変更したファイルのみ stage して git commit -m 'fix: Address PR review comments' && git push origin $$BRANCH"
