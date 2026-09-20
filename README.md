# ssmm

導入済みなら、AWS プロファイル prod の EC2 一覧を表示する。プロファイル名とリージョンは実際の値に置き換える。

```sh
ssmm list -p prod --region ap-northeast-1
```

ssmm は、EC2 の Name タグやインスタンス ID で接続先を検索し、AWS Systems Manager Session Manager 経由でシェル接続・SSH 接続・ファイル転送を行う CLI ツールである。

## 最初の接続

ビルドからシェル接続までを 3 ステップで行う。Go 1.24 以降、AWS CLI v2、Session Manager plugin が必要である。実行環境は Unix を前提とする。

AWS 認証情報と接続先の Session Manager 設定は事前に準備する。SSO を使う場合はログインを済ませる。準備手順の詳細は、[セットアップ](docs/usage/getting-started.md) を参照。

1. リポジトリのルートでビルドし、実行ファイルのあるディレクトリを PATH に追加する。

   ```sh
   go build -o ssmm ./cmd/ssmm
   export PATH="$PWD:$PATH"
   ```

2. 接続先を含むリージョンの選択画面を開く。prod と ap-northeast-1 は実際の値に置き換える。

   ```sh
   ssmm connect -p prod --region ap-northeast-1
   ```

3. ↑↓ キーで running のインスタンスを選び、Enter キーで接続する。

接続先のシェルが開けば完了である。終了時は exit を実行する。

## 検索・接続・転送の例

以下の例では、prod は AWS プロファイル名、web-01 は EC2 の Name タグである。SSH の ec2-user と鍵ファイルのパスも実際の値に置き換える。

### list：EC2 一覧

EC2 の名前、リージョン、インスタンス ID、EC2 状態、SSM 状態を表示する。

```sh
ssmm list -p prod
```

リージョン、タグ、名前の部分一致で絞り込む例を示す。

```sh
ssmm list -p prod --region ap-northeast-1
ssmm list -p prod --tag Environment=production
ssmm list -p prod --filter web
```

スクリプトで扱う場合は JSON、ID だけ必要な場合は id 形式で出力する。

```sh
ssmm list -p prod --output json
ssmm list -p prod --output id
```

検索リージョンを指定・保存していない場合は、アカウントで有効な全リージョンを検索する。

### connect：Session Manager のシェル接続

Name タグまたはインスタンス ID で接続先を指定する。通常のシェル接続には SSH 鍵を使わない。

```sh
ssmm connect web-01 -p prod
ssmm connect i-0123456789abcdef0 -p prod --region ap-northeast-1
```

サブコマンドを省略しても同じ動作になる。接続先も省略すると、一覧から選択できる。

```sh
ssmm web-01 -p prod
ssmm -p prod
```

操作端末が使える場合は、候補が 1 件でも Enter キーで選択する。選択画面を省略する場合は、次のように指定する。

```sh
ssmm connect web-01 -p prod --region ap-northeast-1 --non-interactive
```

この指定では、検索が完了し、全検索範囲の EC2 取得に成功し、候補が 1 件で running の場合だけ接続する。

### ssh：SSH 接続

接続先の SSH サーバー、SSH ユーザー、公開鍵を準備し、ローカルに OpenSSH を用意する。ユーザーと秘密鍵を指定する例を示す。

```sh
ssmm ssh ec2-user@web-01 -p prod --identity-file ~/.ssh/prod.pem
```

ポートを指定する場合は --port を使う。ssmm の -p は AWS プロファイルの指定である。

```sh
ssmm ssh web-01 -p prod --user ec2-user --identity-file ~/.ssh/prod.pem --port 2222
```

### scp：ファイル転送

SSH 接続の準備に加え、接続先の SFTP とローカルの scp が必要である。ローカルのファイルを EC2 へ送信する例を示す。

```sh
ssmm scp -p prod --user ec2-user --identity-file ~/.ssh/prod.pem ./report.csv web-01:/tmp/report.csv
```

EC2 からローカルへ受信する場合は、送信元と宛先を入れ替える。

```sh
ssmm scp -p prod --user ec2-user --identity-file ~/.ssh/prod.pem web-01:/tmp/report.csv ./report.csv
```

ディレクトリ全体を送信する場合は -r を付ける。

```sh
ssmm scp -p prod --user ec2-user --identity-file ~/.ssh/prod.pem -r ./assets web-01:/tmp/assets
```

## 設定・標準 SSH 連携の例

検索リージョンや SSH の既定値は AWS プロファイルごとに保存できる。標準の ssh・scp からの接続も設定できる。

### init：設定の保存

検索リージョン、SSH ユーザー、鍵ファイルを保存する例を示す。init はローカル設定を更新し、AWS の認証情報は作成しない。

```sh
ssmm init -p prod --regions ap-northeast-1 --user ec2-user --identity-file ~/.ssh/prod.pem
```

保存先は ~/.config/ssmm/config.toml である。init は ~/.ssh/ssmm/config と ~/.ssh/config も更新する。

設定用フラグを省略すると、端末上で対話入力できる。

```sh
ssmm init -p prod
```

標準 SSH 連携を有効にすると、NAME.PROFILE.ssmm 形式で接続できる。上記の設定を保存済みなら、次の順に実行する。

1. prod の SSH 連携を有効にする。

   ```sh
   ssmm init -p prod --integration
   ```

2. 標準の ssh で接続する。

   ```sh
   ssh web-01.prod.ssmm
   ```

標準 SSH 連携は対象選択画面を開かず、一意な接続先を要求する。設定項目と更新対象の詳細は、[設定](docs/usage/configuration.md) を参照。

### proxy：OpenSSH の ProxyCommand

proxy は OpenSSH から呼び出して通信を中継する。手動で指定する場合は、次のように ssh の ProxyCommand に組み込む。

```sh
ssh -i ~/.ssh/prod.pem \
  -o 'ProxyCommand=ssmm proxy i-0123456789abcdef0 -p prod --region ap-northeast-1 --port %p' \
  ec2-user@i-0123456789abcdef0
```

インスタンス ID は 2 か所とも実際の値に置き換える。ID とリージョンを直接指定した場合は EC2 検索を省略する。

init で SSH 連携を有効にした場合は、proxy の呼び出しが自動設定される。連携の制約と手動設定の詳細は、[SSH とファイル転送](docs/usage/ssh.md) を参照。

## 補完・ヘルプの例

補完スクリプトとコマンド別ヘルプを利用できる。

### completion：シェル補完

現在の Bash セッションで補完を有効にする例を示す。

```bash
source <(ssmm completion bash)
```

補完スクリプトは、使用するシェルに合わせて次のいずれかで出力する。

```sh
ssmm completion bash
ssmm completion zsh
ssmm completion fish
ssmm completion powershell
```

### help：ヘルプ表示

全体のヘルプと、サブコマンドのヘルプを表示する例を示す。

```sh
ssmm help
ssmm help scp
ssmm ssh --help
```

## 詳細ドキュメント

目的別の参照先を示す。

- 導入と AWS 側の準備は、[セットアップ](docs/usage/getting-started.md) を参照。
- 検索条件、対話操作、出力形式は、[コマンド](docs/usage/commands.md) を参照。
- ビルド、テスト、内部構成は、[開発ガイド](docs/development/README.md) を参照。
