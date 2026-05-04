# favofeeder

お気に入りコンテンツ（アニメ・ゲーム・マンガ・小説）の最新情報を Codex CLI で収集し、Slack に通知するツール。

## アーキテクチャ

```
config/targets.yaml  →  Crawler  →  Codex CLI  →  Parser  →  MySQL
                                                              ↓
                                                         Slack Webhook
```

### パッケージ構成

| パッケージ | 役割 |
|---|---|
| `cmd/favofeeder` | エントリポイント、CLI フラグ |
| `internal/config` | `targets.yaml` の読み込み・バリデーション |
| `internal/codex` | Codex CLI の実行とプロンプト生成 |
| `internal/crawler` | ターゲットごとの取得ループ、クロールログ |
| `internal/parser` | Codex 出力のパース |
| `internal/storage` | MySQL 保存・重複排除・クロールログ・起動時スキーマ自動適用 |
| `internal/notify` | Slack Webhook 通知 |

## 開発コマンド

```bash
# ビルド
go build ./...

# テスト（Hash 系は常時実行、DB 統合テストは DATABASE_TEST_DSN が必要）
go test -race ./...
DATABASE_TEST_DSN="user:pass@tcp(localhost:3306)/favofeeder_test" go test -race ./...

# ドライラン（Codex を呼ばずプロンプトのみ表示）
go run ./cmd/favofeeder -dry-run -v

# 実行
DATABASE_DSN="user:pass@tcp(localhost:3306)/favofeeder" go run ./cmd/favofeeder
```

## CLI フラグ

| フラグ | デフォルト | 説明 |
|---|---|---|
| `-config` | `config/targets.yaml` | ターゲット設定ファイルのパス |
| `-db` | `$DATABASE_DSN` | MySQL DSN（例: `user:pass@tcp(host:3306)/dbname`） |
| `-codex` | `codex` | Codex CLI バイナリのパス |
| `-model` | `gpt-5.5` | Codex が使用するモデル |
| `-dry-run` | `false` | プロンプトを表示するだけ（保存・通知なし） |
| `-since-days` | `30` | 初回実行時に遡る日数（2回目以降は前回クロール時刻を使用） |
| `-v` | `false` | デバッグログを有効化 |

`parseTime=true` は DSN に指定がなくても自動付与されます。

## データベース

MySQL を使用。起動時に `CREATE TABLE IF NOT EXISTS` でスキーマを自動適用するため、テーブルの DDL を手動で流す必要はない。

接続先は `DATABASE_DSN` 環境変数または `-db` フラグで指定する。

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
# → DATABASE_DSN を実際の値に編集

# 4. Codex Plus ログイン（初回のみ）
#    表示された URL を手元のブラウザで開いてコードを入力する
codex login --device-auth

# 5. 実行
docker compose run --rm favofeeder
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
- `.env` — DATABASE_DSN など機密設定
