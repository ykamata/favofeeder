# favofeeder

お気に入りコンテンツ（アニメ・ゲーム・マンガ・小説）の最新情報を Codex CLI で収集し、Slack に通知するツール。

## アーキテクチャ

```
config/targets.yaml  →  Crawler  →  Brave Search API  →  Codex CLI  →  Parser  →  MySQL（本番）
                                                                                  │  SQLite（ローカル）
                                                                                  ↓
                                                                             Slack Webhook
```

### パッケージ構成

| パッケージ | 役割 |
|---|---|
| `cmd/favofeeder` | エントリポイント、CLI フラグ |
| `internal/config` | `targets.yaml` の読み込み・バリデーション |
| `internal/search` | Brave Search API クライアント（web 検索結果を取得） |
| `internal/codex` | Codex CLI の実行とプロンプト生成（検索結果を埋め込む） |
| `internal/crawler` | ターゲットごとの取得ループ、クロールログ |
| `internal/parser` | Codex 出力のパース |
| `internal/storage` | DB 保存・重複排除・クロールログ・起動時スキーマ自動適用（MySQL / SQLite） |
| `internal/notify` | Slack Webhook 通知 |

### 処理フロー

1. `crawler.runSearches` が Brave Search API を呼び出し（3クエリ: 最新情報・ニュース・アップデート）。取得ウィンドウ（既定=昨日・今日）を `freshness=YYYY-MM-DDtoYYYY-MM-DD` の日付レンジで指定し、新着のみを取得する
2. 検索結果（重複除去・日付フィルタ済み）を `codex.BuildPrompt` に渡す
3. Codex CLI は渡された検索結果のみを使い JSON を返す（`web_search` ツール使用禁止）
4. `parser.Parse` で JSON をパース → 日付フィルタ → DB 保存 → Slack 通知

## 開発コマンド

```bash
# ビルド
go build ./...

# テスト（DATABASE_TEST_DSN 未設定時は SQLite in-memory で実行）
go test -race ./...
DATABASE_TEST_DSN="user:pass@tcp(localhost:3306)/favofeeder_test" go test -race ./...

# ドライラン（Codex を実際に呼び出してレスポンスを確認、保存・通知なし）
go run ./cmd/favofeeder -local -dry-run -v

# ローカル実行（SQLite）
go run ./cmd/favofeeder -local

# 本番実行（MySQL）
DATABASE_DSN="user:pass@tcp(localhost:3306)/favofeeder" go run ./cmd/favofeeder
```

## CLI フラグ

| フラグ | デフォルト | 説明 |
|---|---|---|
| `-config` | `config/targets.yaml` | ターゲット設定ファイルのパス |
| `-db` | `$DATABASE_DSN` | MySQL DSN または SQLite ファイルパス |
| `-local` | `false` | SQLite を使用（ローカル開発用）。`-db` 未指定時は `favofeeder.db` |
| `-codex` | `codex` | Codex CLI バイナリのパス |
| `-model` | `gpt-5.5` | Codex が使用するモデル |
| `-dry-run` | `false` | Codex を呼び出してレスポンスを表示するが、保存・通知はしない |
| `-since-days` | `2` | 遡る日数の固定ウィンドウ（`2` = 昨日・今日）。この範囲を毎回取得し、重複は URL ハッシュで排除 |
| `-v` | `false` | デバッグログを有効化（プロンプト内容を出力） |
| `-brave-key` | `$BRAVE_API_KEY` | Brave Search API キー（未設定時は web 検索スキップ） |

`parseTime=true` は MySQL DSN に指定がなくても自動付与されます。

## 環境変数

| 変数 | 必須 | 説明 |
|---|---|---|
| `DATABASE_DSN` | 本番のみ | MySQL DSN（例: `user:pass@tcp(mysql:3306)/favofeeder`） |
| `BRAVE_API_KEY` | 任意 | Brave Search API キー（未設定時は web 検索なしで動作） |

`.env` ファイルに設定するとローカル開発時に自動読み込みされます（`godotenv`）。

## データベース

本番（Ubuntu）は MySQL、ローカル開発は SQLite を使用。起動時に `CREATE TABLE IF NOT EXISTS` でスキーマを自動適用する。

| 環境 | 起動方法 |
|---|---|
| ローカル開発 | `go run ./cmd/favofeeder -local -dry-run -v` |
| 本番 | `DATABASE_DSN="..." docker compose run --rm favofeeder` |

接続先は `DATABASE_DSN` 環境変数または `-db` フラグで指定する（MySQL の場合）。

### 事前準備（初回のみ）

アプリが接続する **Database 自体は事前に作成が必要**。MySQL に管理者権限を持つユーザーで接続して実行する。

```sql
CREATE DATABASE IF NOT EXISTS favofeeder
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

-- アプリ用ユーザーが別途必要な場合
CREATE USER 'favofeeder'@'%' IDENTIFIED BY 'password';
GRANT ALL PRIVILEGES ON favofeeder.* TO 'favofeeder'@'%';
FLUSH PRIVILEGES;
```

## Docker（サーバー運用）

Ubuntu サーバー上では Docker Compose で運用する。

- ネットワーク `nekolog_nekolog-network`（external）を経由して MySQL コンテナに接続
- Codex CLI（Plus プラン）の認証情報はホストの `~/.codex/` をマウント

### サーバー初回セットアップ

```bash
# 1. リポジトリを clone
git clone <repo> && cd favofeeder

# 2. targets.yaml を配置（gitignore 済みのため手動コピー）
cp config/targets.yaml.sample config/targets.yaml
# → 実際の内容に編集

# 3. .env を作成
cp .env.example .env
# → DATABASE_DSN と BRAVE_API_KEY を実際の値に編集

# 4. Codex Plus ログイン（初回のみ）
#    表示された URL を手元のブラウザで開いてコードを入力する
codex login --device-auth

# 5. 実行
docker compose run --rm favofeeder
```

### サーバー更新手順

コード変更後は `scripts/deploy.sh` を使う。

```bash
# 通常のデプロイ（pull + build）
./scripts/deploy.sh

# ビルド後に dry-run で動作確認
./scripts/deploy.sh --dry-run

# ビルド後にそのまま本番実行
./scripts/deploy.sh --run
```

`config/targets.yaml` だけ変更した場合はビルド不要（ボリュームマウントのため）。

```bash
# targets.yaml をローカルからコピーして即時反映
scp config/targets.yaml ubuntu-server:/opt/myapp/favofeeder/config/targets.yaml
ssh ubuntu-server sudo systemctl start favofeeder.service
```

### 定期実行（systemd timer）

`deploy/` 以下のユニットファイルを使って JST 9:00 / 18:00 に定期実行する。

```bash
# ユニットファイルをインストール
sudo cp deploy/favofeeder.service /etc/systemd/system/
sudo cp deploy/favofeeder.timer   /etc/systemd/system/

# 有効化・起動
sudo systemctl daemon-reload
sudo systemctl enable --now favofeeder.timer

# 状態確認
systemctl status favofeeder.timer
systemctl list-timers favofeeder.timer

# ログ確認
journalctl -u favofeeder.service
```

## 設定ファイル

`config/targets.yaml` は `.gitignore` に含まれており、**リポジトリに追跡されない**。
テンプレートは `config/targets.yaml.sample` を参照。

```yaml
slack:
  webhooks:
    - webhook_url: "https://hooks.slack.com/services/..."
      # titles を省略すると全ターゲットが対象
      # titles: ["タイトルA"]

targets:
  - title: "タイトルA"
    category: game          # anime / game / manga / novel
    sources:
      - type: x_account
        account: "@example"
      - type: website
        url: "https://example.com/news/"
```

## 機密ファイル（.gitignore 済み）

- `config/targets.yaml` — 実際のターゲット情報（個人情報含む）
- `.env` — DATABASE_DSN・BRAVE_API_KEY など機密設定
