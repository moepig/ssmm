# ssmm

ssmm は、EC2 の Name タグやインスタンス ID で接続先を検索し、AWS Systems Manager Session Manager 経由でシェル接続・SSH 接続・ファイル転送を行う CLI ツールである。接続先を一覧から選択でき、複数のリージョンをまとめて検索できる。

接続に必要な値はコマンドラインで指定できる。繰り返し使う AWS プロファイル名、検索リージョン、SSH の既定値は、ssmm プロファイルとして保存できる。

## 目次

各項目へのリンクを、以下に示す。

- [動作環境](#動作環境)
- [インストール](#インストール)
- [使い方](#使い方)
  - [プロファイルなしでの接続](#プロファイルなしでの接続)
  - [プロファイルによる接続設定の再利用](#プロファイルによる接続設定の再利用)
- [コマンド](#コマンド)
- [設定ファイルと環境変数](#設定ファイルと環境変数)
- [更新とアンインストール](#更新とアンインストール)
- [詳細ドキュメント](#詳細ドキュメント)

## 動作環境

Linux と macOS を対象とし、それぞれ amd64 と arm64 向けのバイナリを配布する。Windows ネイティブには対応していない。接続対象は商用 AWS の EC2 インスタンスである。

操作に必要なコマンドを、以下にまとめる。

| 操作 | 必要なコマンド |
| --- | --- |
| EC2 の検索と一覧表示 | ssmm |
| Session Manager のシェル接続 | ssmm、AWS CLI v2、Session Manager plugin |
| SSH 接続 | 上記に加えて OpenSSH の ssh |
| ファイル転送 | 上記に加えて OpenSSH 9.0 以降の scp |
| ソースからのビルド | Go 1.24 以降 |

AWS の認証情報と、接続先の SSM Agent・IAM 権限・Session Manager エンドポイントへの通信経路を事前に準備する。SSO を使う場合は、接続前にログインを済ませる。通常のシェル接続には SSH サーバーや SSH 鍵は不要である。

依存コマンドと AWS 側の準備の詳細は、[セットアップ](docs/usage/getting-started.md) を参照。

## インストール

リリースバイナリを配置する方法と、ソースからビルドする方法がある。以降の操作例は、ssmm が PATH の通ったディレクトリにあることを前提とする。

### リリースバイナリ

[GitHub Releases](https://github.com/moepig/ssmm/releases) から、OS とアーキテクチャに合う ssmm_VERSION_OS_ARCH 形式のバイナリと checksums.txt を取得する。OS 名は Linux が linux、macOS が darwin である。

ダウンロードしたバイナリの SHA-256 を checksums.txt と照合し、ssmm に改名する。実行権限を付けて ~/.local/bin に配置する例を、以下に示す。

```sh
mkdir -p ~/.local/bin
chmod +x ./ssmm
mv ./ssmm ~/.local/bin/ssmm
export PATH="$HOME/.local/bin:$PATH"
ssmm --version
```

継続して使う場合は、使用するシェルの起動設定にも PATH を追加する。

### ソースからのビルド

リポジトリを取得し、ビルドする例を、以下に示す。

```sh
git clone https://github.com/moepig/ssmm.git
cd ssmm
go build -o ssmm ./cmd/ssmm
mkdir -p ~/.local/bin
install -m 755 ./ssmm ~/.local/bin/ssmm
export PATH="$HOME/.local/bin:$PATH"
ssmm --help
```

## 使い方

ssmm プロファイルを作成せずに接続する方法と、接続設定を保存して再利用する方法を示す。AWS プロファイルは AWS の認証設定の名前であり、ssmm プロファイルとは別の設定である。

以下の例では、company-prod は既存の AWS プロファイル名、ap-northeast-1 は接続先のリージョン、web-01 は EC2 の Name タグである。それぞれ実際の値に置き換える。

### プロファイルなしでの接続

ssmm プロファイルの作成や init の実行は不要である。AWS プロファイルとリージョンを直接指定して、接続先の選択画面を開く。

```sh
ssmm -p company-prod -r ap-northeast-1
```

↑↓ キーで running のインスタンスを選び、Enter キーで接続する。接続先のシェルを終了する場合は exit を実行する。

Name タグや EC2 タグで接続先を絞る例を、以下に示す。

```sh
ssmm web-01 -p company-prod -r ap-northeast-1
ssmm -p company-prod -r ap-northeast-1 -t Environment=production
```

環境変数や IAM ロールなどで AWS 認証情報を取得できる場合は、-p を省略できる。AWS プロファイルも指定せずに接続する例を、以下に示す。

```sh
ssmm -r ap-northeast-1
```

### プロファイルによる接続設定の再利用

繰り返し使う AWS プロファイルと検索リージョンを、ssmm プロファイルに保存する。prod という名前で保存し、その設定で接続する例を、以下に示す。

```sh
ssmm init -s prod -p company-prod -r ap-northeast-1
ssmm -s prod
ssmm web-01 -s prod
```

設定は ~/.config/ssmm/config.toml に保存される。init は AWS の認証情報を作成しない。保存後も、使用する ssmm プロファイルを -s で指定する。

実行時の -p と -r は保存値より優先する。一時的に検索リージョンを変更する例を、以下に示す。

```sh
ssmm -s prod -r us-east-1
```

SSH のユーザーと鍵も保存できる。接続先の SSH サーバーと公開鍵を準備したうえで、標準の ssh・scp から接続する設定例を、以下に示す。ec2-user と鍵ファイルのパスは実際の値に置き換える。

```sh
ssmm init --ssh -s prod --user ec2-user --identity-file ~/.ssh/prod.pem
ssmm ssh-config create -s prod
ssh web-01.prod.ssmm
scp ./report.csv web-01.prod.ssmm:/tmp/report.csv
```

init --ssh は ssmm の設定だけを更新する。ssh-config create は ~/.ssh/ssmm/config を生成し、~/.ssh/config に Include を設定する。NAME.PROFILE.ssmm の PROFILE は ssmm プロファイル名である。標準 SSH 連携では選択画面を開かず、接続先が一意に決まる必要がある。

## コマンド

検索、接続、転送、設定管理の代表的な使い方を示す。各コマンドのフラグは ssmm help コマンド名 で確認できる。

### list

EC2 の名前、リージョン、インスタンス ID、EC2 状態、SSM 状態を表示する。タグや部分一致フィルタで絞り込む例を、以下に示す。

```sh
ssmm list -p company-prod -r ap-northeast-1
ssmm list -s prod -t Environment=production
ssmm list -s prod --filter web
```

検索リージョンをコマンドラインにも ssmm プロファイルにも指定しない場合は、アカウントで有効な全リージョンを検索する。

JSON またはインスタンス ID だけを出力する例を、以下に示す。

```sh
ssmm list -s prod --output json
ssmm list -s prod --output id
```

### connect

Session Manager のシェルに接続する。サブコマンドを省略した ssmm と同じ動作である。接続先に Name タグまたはインスタンス ID を指定する例を、以下に示す。

```sh
ssmm connect web-01 -s prod
ssmm connect i-0123456789abcdef0 -p company-prod -r ap-northeast-1
```

TARGET を指定した場合は、全検索範囲の EC2 取得に成功し、候補が 1 件で running なら選択画面を開かずに接続する。0 件または複数件ならエラーになる。TARGET を省略した場合は、操作端末で接続先を選択する。TARGET を省略して選択画面も開かない例を、以下に示す。

```sh
ssmm connect -s prod --filter web-01 --non-interactive
```

--non-interactive では、全検索範囲の EC2 取得に成功し、候補が 1 件で running の場合だけ接続する。

### ssh・scp（experimental）

Session Manager を経由して SSH 接続・ファイル転送を行う。接続先の SSH サーバー、SSH ユーザー、公開鍵を準備する。ファイル転送には接続先の SFTP も必要である。

ssmm プロファイルを使わずに SSH 接続する例を、以下に示す。

```sh
ssmm ssh ec2-user@web-01 -p company-prod -r ap-northeast-1 --identity-file ~/.ssh/prod.pem
```

prod に保存済みの SSH 設定を使い、接続、ファイルの送信・受信、ディレクトリの送信を行う例を、以下に示す。

```sh
ssmm ssh web-01 -s prod
ssmm scp -s prod ./report.csv web-01:/tmp/report.csv
ssmm scp -s prod web-01:/tmp/report.csv ./report.csv
ssmm scp -s prod --recursive ./assets web-01:/tmp/assets
```

ssmm の -p は AWS プロファイル、-r はリージョンの指定である。SSH ポートには --port、ディレクトリ転送には --recursive を使う。

### init

ssmm プロファイルを作成・更新する。設定用フラグを省略すると、端末上で対話入力できる。対話入力と複数リージョンの保存の例を、以下に示す。

```sh
ssmm init -s prod
ssmm init -s prod --regions ap-northeast-1,us-east-1
```

SSH の既定値を設定する場合は --ssh を付ける。設定項目の詳細は、[設定](docs/usage/configuration.md) を参照。

### ssh-config・proxy

ssh-config は標準の ssh・scp 用の設定を作成・削除する。SSH の既定値を変更した場合は create を再実行して反映する。作成・削除のコマンドを、以下に示す。

```sh
ssmm ssh-config create -s prod
ssmm ssh-config delete -s prod
```

proxy は OpenSSH の ProxyCommand として通信を中継する。ssh-config create が呼び出しを自動設定する。手動で組み込む方法と連携の制約の詳細は、[SSH とファイル転送](docs/usage/ssh.md) を参照。

### completion・help

シェル補完とコマンド別ヘルプを利用できる。現在の Bash セッションで補完を有効にする例を、以下に示す。

```bash
source <(ssmm completion bash)
```

補完スクリプトは bash、zsh、fish、powershell 向けに出力できる。ヘルプの表示例を、以下に示す。

```sh
ssmm help
ssmm help scp
ssmm ssh --help
```

## 設定ファイルと環境変数

ssmm プロファイルは ~/.config/ssmm/config.toml に保存する。検索・接続では、-s または標準 SSH 連携のホスト名でプロファイルを指定した場合だけ読み込む。

AWS プロファイルと検索リージョンの決定順序を、以下にまとめる。

| 項目 | 優先順位 |
| --- | --- |
| AWS プロファイル | -p → ssmm プロファイルの aws_profile → AWS_PROFILE → 通常の AWS 認証情報取得 |
| 検索リージョン | -r → ssmm プロファイルの regions → アカウントで有効な全リージョン |

AWS_PROFILE は AWS プロファイルを選択する環境変数であり、ssmm プロファイルを選択しない。AWS の共有設定や環境変数のリージョンは、EC2 の検索範囲を制限しない。

設定ファイルの形式と各設定値の詳細は、[設定](docs/usage/configuration.md) を参照。

## 更新とアンインストール

更新する場合は、新しいリリースバイナリを取得し、インストール済みの ssmm を置き換える。ソースから導入した場合は、ソースを更新して再ビルドする。

アンインストールする場合は、標準 SSH 連携を有効にした各プロファイルで ssh-config delete を実行してから、ssmm の実行ファイルを削除する。保存済みのプロファイルも不要なら ~/.config/ssmm/config.toml を削除する。

## 詳細ドキュメント

目的別の参照先を、以下にまとめる。

| 目的 | ドキュメント |
| --- | --- |
| 依存コマンド、AWS 側の準備、接続できない場合の確認 | [セットアップ](docs/usage/getting-started.md) |
| 検索条件、対話操作、出力形式 | [コマンド](docs/usage/commands.md) |
| プロファイル、設定ファイル、設定の優先順位 | [設定](docs/usage/configuration.md) |
| SSH 接続、ファイル転送、標準 SSH 連携 | [SSH とファイル転送](docs/usage/ssh.md) |
| ビルド、テスト、内部構成 | [開発ガイド](docs/development/README.md) |
