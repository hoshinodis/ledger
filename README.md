# ledger

AIエージェントと人間が共有するローカルファーストなIssue管理ツール。

Project Context(長期知識)・Issue(作業単位)・Comment(作業履歴)をSQLiteに蓄積し、新しいセッションが **Session Bootstrap** でプロジェクトの状況を一回の呼び出しで把握できるようにする。REST API、MCP、CLIの3つの操作面が同じservice層を共有する。

## 構成

- Backend: Go (single binary)
- Database: SQLite (modernc.org/sqlite, cgoなし / WAL mode)
- Frontend: TypeScript / React (Vite)
- AI連携: MCP Server (Streamable HTTP, 公式 Go SDK)
- 起動: Docker Compose

```
service層 ─┬─ REST API  http://<host>:8080/api/*
           ├─ MCP       http://<host>:8080/mcp
           ├─ Web UI    http://<host>:8080/      (web/dist を静的配信)
CLI ───────┴─ REST に接続するクライアント (ledger コマンド)
```

## 起動

### Docker Compose(推奨)

```sh
docker compose up -d --build
# UI:      http://localhost:8080
# API:     http://localhost:8080/api/projects
# MCP:     http://localhost:8080/mcp
```

SQLiteデータは `ledger-data` ボリューム(`/data/ledger.db`)に永続化される。バックアップはボリューム内のファイルをコピーするだけ:

```sh
docker compose cp ledger:/data/ledger.db ./backup.db
```

### ローカルで直接起動

```sh
go build -o ledger ./cmd/ledger
ledger serve                       # REST + MCP + UI(web/dist がある場合)
ledger serve -db ./ledger.db -addr 0.0.0.0:8080
```

Web UIをローカルでも使う場合は事前にビルド:

```sh
cd web && npm install && npm run build
```

## Session Bootstrap

新しいセッションは最初にこれを呼ぶ:

```
get_project_bootstrap(project_id, session_id)
GET /api/projects/{id}/bootstrap?session_id={sid}
ledger bootstrap <project_id> -s <session_id>
```

返却内容: プロジェクト概要とMarkdownのProject Context、指定セッションに割り当て済みの未完了Issue、オープンなIssue、直近の完了Issue、直近のComment。全履歴は返さず、作業開始に必要な概要に限定している。

## MCP接続

Streamable HTTP。サーバが動いているホストを指定する。

**Claude Code:**

```sh
claude mcp add --transport http ledger http://localhost:8080/mcp
```

**Codex (`~/.codex/config.toml`):**

```toml
[mcp_servers.ledger]
url = "http://localhost:8080/mcp"
```

**ツール一覧(15):**

| 種別 | ツール |
|---|---|
| Project | `list_projects` `get_project` `update_project_context` `get_project_bootstrap` |
| Issue | `list_issues` `get_issue` `create_issue` `update_issue` `complete_issue` `search_issues` |
| Session | `list_my_issues` `assign_issue` `unassign_issue` |
| Comment | `add_comment` `list_comments` |

`complete_issue` は `body`(結果コメント)が必須。`update_issue` では `status: "done"` は拒否されるため、完了は必ず `complete_issue` で結果コメントを残す。

## CLI(MCPが使えない環境向け)

サーバのREST APIを叩くクライアント。`LEDGER_URL` 環境変数または `-server` フラグでサーバを指定(既定 `http://127.0.0.1:8080`)。

```sh
# サーバURLは毎回環境変数で
export LEDGER_URL=http://macmini:8080

ledger projects create -n "demo" -d "説明"
ledger bootstrap 1 -s sess-a                 # セッション開始時の定番コマンド
ledger issues create -p 1 -t "ログイン修正" -b "背景と完了条件…" -l bug -s sess-a
ledger issues list-my -s sess-a
ledger issues complete 1 -f result.md -a claude-code -s sess-a   # 結果コメントはファイルから
ledger comments add 1 -b "調査メモ…" -a tester
ledger issues search -q "ログイン" -p 1
```

`-f -` で標準入力からMarkdownを渡せる。`-json` で生JSON出力。

## REST API

| METHOD | PATH | 内容 |
|---|---|---|
| GET | `/api/health` | 疎通確認 |
| GET / POST | `/api/projects` | 一覧 / 作成 |
| GET / PATCH | `/api/projects/{id}` | 取得 / 部分更新 |
| GET / PUT | `/api/projects/{id}/context` | Context取得 / Markdown置換 |
| GET | `/api/projects/{id}/bootstrap` | Session Bootstrap |
| GET / POST | `/api/projects/{id}/labels` | ラベル一覧 / 作成 |
| DELETE | `/api/labels/{id}` | ラベル削除 |
| GET / POST | `/api/issues` | 一覧(フィルタ) / 作成 |
| GET / PATCH | `/api/issues/{id}` | 取得 / 部分更新 |
| POST | `/api/issues/{id}/complete` | 完了(結果コメント必須) |
| POST | `/api/issues/{id}/assign` | 担当セッション設定 |
| POST | `/api/issues/{id}/unassign` | 担当解除 |
| GET / POST | `/api/issues/{id}/comments` | コメント一覧 / 追加 |
| GET / POST | `/api/issues/{id}/relations` | 関連一覧 / 追加(`relates_to` `blocks`) |
| DELETE | `/api/issues/{id}/relations` | 関連削除 |

一部更新はPATCHで、渡したフィールドだけ変更される。エラーは問題の種類で4xxにマップされる(404不在 / 400検証NG / 409状態不一致)。

## セッション運用の想定フロー

1. 新しいセッションが `get_project_bootstrap` を呼ぶ(Project Context + 自分のIssue + 最近の状況)
2. 作業対象のIssueに `assign_issue` で担当登録(セッションIDはエージェント側で発行)`。
3. 作業中の判断・調査は `add_comment` で残す
4. 完了時に `complete_issue` で結果コメントを付けてDone
5. 次のセッションは 1. に戻る

セッションIDは本ツールでは不透明な文字列として扱い、セッションの実行管理はしない。

## 開発

```sh
go test ./...          # service/MCP のルールテスト
go vet ./...           # 静的検査
cd web && npm run check && npm run build   # UI型チェックとビルド
```

スキーマは `internal/repository/schema.sql`(起動時に自動適用)。

## スコープ外(意図的にやっていない)

- ユーザー/権限管理、認証(ローカル運用のみを想定)
- Sprint/Epic管理、複雑なワークフロー
- Git/GitHub連携、ベクトル検索、LLM実行
- セッションの実行状態や会話履歴の保存(本ツールは事実と判断を蓄積する)
