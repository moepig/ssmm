# セットアップ

本ドキュメントは、ssmm のビルド、実行環境と AWS の準備、最初の接続手順を説明する。

## ローカル環境

操作に必要なコマンドを、以下にまとめる。

| 操作 | 必要なコマンド |
| --- | --- |
| ソースからのビルド | Go 1.24 以降 |
| EC2 の検索と一覧表示 | `ssmm` |
| Session Manager 接続 | `ssmm`、`aws`、`session-manager-plugin` |
| SSH 接続 | 上記に加えて `ssh` |
| ファイル転送 | 上記に加えて `scp` |

接続用には AWS CLI v2 を用意する。SFTP によるファイル転送には OpenSSH 9.0 以降を用いる。ssmm は依存コマンドの存在を確認するが、バージョンの検証は行わない。

`scp` の転送方式とオプションの詳細は、[OpenSSH の scp マニュアル](https://man.openbsd.org/scp.1) を参照。

## ビルド

リポジトリのルートで、次のコマンドを実行する。

```sh
go build -o ssmm ./cmd/ssmm
./ssmm --help
```

生成した `ssmm` を `PATH` の通ったディレクトリへ配置する。以降の例は `ssmm` というコマンド名で実行できることを前提とする。

> [!NOTE]
> `.git` の情報が不完全なソースコピーで `error obtaining VCS status` が発生する場合は、`go build -buildvcs=false -o ssmm ./cmd/ssmm` を使う。

## AWS の準備

### 認証情報

AWS SDK for Go v2 と AWS CLI の両方で解決できる認証情報を用意する。ssmm の設定にはアクセスキーやセッショントークンを保存しない。SSO などの事前ログインは ssmm の実行前に済ませる。

プロファイルの選択順序と環境変数の扱いは、[設定の AWS プロファイル](configuration.md#aws-プロファイル) を参照。

### 接続先

接続先に SSM Agent、Session Manager 用のインスタンス権限、Session Manager エンドポイントへの通信経路を用意する。通常のシェル接続では SSH サーバーや SSH 鍵は使わない。

AWS 側の設定手順は、[Session Manager のセットアップ](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-getting-started.html) を参照。

### 実行元の権限

ssmm が検索に使う API と用途を、以下にまとめる。

| IAM アクション | 用途 |
| --- | --- |
| `ec2:DescribeRegions` | 有効なリージョンの取得。`--region` 指定時も使用する |
| `ec2:DescribeInstances` | 対象リージョンの EC2 検索 |
| `ssm:DescribeInstanceInformation` | 検索結果に付加する SSM 状態の取得 |

セッション接続には、対象インスタンスとセッションドキュメントへの `ssm:StartSession` など、Session Manager の操作に必要な権限を追加する。SSH 接続では `AWS-StartSSHSession` の許可も必要である。セッションの終了や暗号化に必要な権限は AWS の構成に従う。

権限設定の詳細は、[Session Manager の IAM ポリシー例](https://docs.aws.amazon.com/systems-manager/latest/userguide/getting-started-restrict-access-quickstart.html) を参照。

## 最初の検索と接続

以下では、既存の AWS プロファイル `prod` と、接続先を含むリージョン `ap-northeast-1` を使う。

1. `ssmm list -p prod --region ap-northeast-1` で EC2 一覧を取得する。
2. `ssmm connect -p prod --region ap-northeast-1` で選択画面を開く。
3. ↑↓ キーで `running` のインスタンスを選び、Enter キーで接続する。

検索リージョンを保存する場合は、次のコマンドを実行する。`init` は AWS の認証情報を作成せず、ローカルの設定ファイルを更新する。

```sh
ssmm init -p prod --regions ap-northeast-1
ssmm list -p prod
ssmm connect web-01 -p prod
```

`init` が更新するファイルと対話入力の詳細は、[設定](configuration.md) を参照。

## 接続できない場合の確認

症状ごとの確認点を、以下にまとめる。

| 症状 | 確認点 |
| --- | --- |
| プロファイルが見つからない | AWS の共有設定・認証情報ファイルに指定名が存在するか |
| リージョン取得が失敗する | 認証情報、`ec2:DescribeRegions`、API への通信経路 |
| `EC2 inventory is incomplete` | 標準エラー出力の対象リージョン、`ec2:DescribeInstances`、検索範囲 |
| SSM が `Unknown` | SSM 情報の取得権限と通信。状態の取得に失敗しても EC2 一覧は表示できる |
| 候補が 0 件、または複数件 | Name の大文字・小文字、リージョン、タグ、部分一致フィルタ |
| 停止中のインスタンスを選択できない | EC2 が `running` か。ssmm は起動操作を行わない |
| `aws` や plugin が見つからない | `PATH` と依存コマンドのインストール |
| 一覧に表示されるが接続に失敗する | SSM Agent、セッション権限、接続先の通信、AWS CLI が出すエラー |

SSM 状態の意味と検索失敗時の動作は、[コマンド](commands.md) を参照。
