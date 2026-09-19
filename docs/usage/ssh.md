# SSH とファイル転送

本ドキュメントは、Session Manager を経由する SSH 接続とファイル転送、標準の OpenSSH コマンドとの連携を説明する。

## 接続の前提

通常の Session Manager 接続に必要な準備に加えて、接続先に SSH サーバー、SSH ユーザー、認証用の公開鍵を用意する。SSH 用の Session Manager 接続には SSM Agent 2.3.672.0 以降と Session Manager plugin 1.1.23.0 以降が必要である。転送には接続先の SFTP サブシステムも必要である。

SSH 接続の権限と接続先の準備の詳細は、[AWS の SSH 接続設定](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-getting-started-enable-ssh-connections.html) を参照。

## ssmm による SSH 接続

`ssmm ssh` は対象を検索し、選択したインスタンスの ID とリージョンを固定して OpenSSH を起動する。標準 SSH 連携の有効化は不要である。

SSH ユーザーと鍵を指定する例を、以下に示す。

```sh
ssmm ssh ec2-user@web-01 -p prod --identity-file ~/.ssh/prod.pem
ssmm ssh web-01 -p prod --user ec2-user --port 2222
ssmm ssh -p prod --filter web
```

対象の検索と対話選択の詳細は、[コマンド](commands.md) を参照。

### SSH の設定値

ssmm が OpenSSH へ渡す設定値の決定順序を、以下にまとめる。

| 項目 | 優先順位 |
| --- | --- |
| ユーザー | `USER@TARGET` または `--user` → プロファイルの `ssh.user` → OpenSSH の設定 |
| 鍵 | `--identity-file` → プロファイルの `ssh.identity_file` → OpenSSH の設定 |
| ポート | 非 0 の `--port` → 非 0 の `ssh.port` → 22 |

`USER@TARGET` と `--user` を併用する場合は同じ値にする。異なる値はエラーになる。ユーザー名は AMI から自動判定しない。

ssmm のラッパーが作るトンネルの既定ポートは 22 である。別ポートを使う場合は `--port` または `ssh.port` に明示する。標準 SSH 連携では OpenSSH が決定したポートを proxy へ渡す。

既存の OpenSSH 設定も読み込まれる。`--identity-file` は ssmm が追加する鍵の指定であり、ほかの `IdentityFile` や ssh-agent の鍵を除去する操作ではない。

### ラッパーの制約

`ssmm ssh` は任意の SSH オプションやリモートコマンドを渡す構文を提供しない。`-L`、`-o`、リモートコマンドなどが必要な場合は、標準 SSH 連携を有効にして `ssh` を直接使う。

## ファイル転送

`ssmm scp` のリモート指定は `[USER@]TARGET:PATH` である。ローカルから 1 台への送信と、1 台からローカルへの受信に対応する。

ファイル送信、受信、ディレクトリ送信の例を、以下に示す。ここでは `prod` の SSH ユーザーと鍵を設定済みとする。

```sh
ssmm scp -p prod ./report.csv web-01:/tmp/report.csv
ssmm scp -p prod web-01:/var/log/app.log ./app.log
ssmm scp -p prod -r ./assets web-01:/tmp/assets
```

複数ファイルの送受信の例を、以下に示す。受信元は同一の TARGET とユーザーにそろえる。

```sh
ssmm scp -p prod ./a.txt ./b.txt web-01:/tmp/
ssmm scp -p prod web-01:/tmp/a.txt web-01:/tmp/b.txt ./downloads/
```

対応するオプションは `--profile` / `-p`、`--region`、`--user`、`--identity-file`、`--port`、`--recursive` / `-r`、`--non-interactive` である。`--filter`、`--tag`、`-O` や任意の `scp` オプションは受け付けない。

リモート間転送、ローカル間転送、`scp://` URI、IPv6 形式はサポートしない。コロンを含むローカルファイル名には `./` を付ける。空白やシェルの特殊文字を含むパスはシェルで引用する。リモートパスのワイルドカードや `~/` は OpenSSH と SFTP サーバーの解釈に従う。

## 標準 SSH 連携

`init` で SSH 連携を有効化すると、`NAME.PROFILE.ssmm` または `INSTANCE_ID.PROFILE.ssmm` を標準の `ssh`・`scp` で指定できる。

`prod` の SSH 連携を有効にする例を、以下に示す。

```sh
ssmm init -p prod --regions ap-northeast-1 --user ec2-user --identity-file ~/.ssh/prod.pem --integration
ssh web-01.prod.ssmm
scp ./report.csv web-01.prod.ssmm:/tmp/report.csv
ssh web-01.prod.ssmm uname -a
```

生成設定は `~/.ssh/ssmm/config` に保存され、`~/.ssh/config` の `Include` から読み込まれる。`ProxyCommand` が `ssmm proxy` を起動するため、これらのホスト名を DNS に登録する必要はない。

名前解決は常に非対話であり、全検索範囲の EC2 取得が成功し、候補が 1 件で `running` の場合だけ接続する。Name が重複する場合は、プロファイルのリージョンを限定するかインスタンス ID を使う。

ホスト名内のプロファイルは、小文字英数字とハイフンからなる 1〜63 文字のラベルに限る。Name は同じ制約を満たすラベルをドットで連結できる。各ラベルの先頭と末尾は英数字にする。大文字、空白、アンダースコアを含む Name は、この形式では指定できない。

SSH 連携を無効にする例を、以下に示す。

```sh
ssmm init -p prod --integration=false
```

## proxy

`ssmm proxy` は OpenSSH の `ProxyCommand` として使用するコマンドである。標準入力・標準出力は接続の通信に使い、選択画面を開かない。

受け付ける公開フラグは `--profile` / `-p`、`--region`、`--port` である。ポートの既定値は 22 であり、ssmm 設定の `ssh.port` を自動では使わない。

インスタンス ID と `--region` の両方を直接指定すると、EC2 検索と ssmm 設定の読み込みを省略して接続する。この経路では EC2 の状態や検索範囲を確認しない。Name や `.ssmm` ホスト名を指定すると検索を行い、一意な対象を要求する。

手動の SSH 設定に組み込む例を、以下に示す。インスタンス ID、リージョン、ユーザー、鍵を実際の値へ置き換える。

```sshconfig
Host web-via-ssmm
    User ec2-user
    IdentityFile ~/.ssh/prod.pem
    ProxyCommand ssmm proxy i-0123456789abcdef0 --profile prod --region ap-northeast-1 --port %p
```

## ホスト鍵とセッション

通常の OpenSSH のホスト鍵検証を使用する。ラッパーは `ssmm-REGION-INSTANCE_ID-PORT` を `HostKeyAlias` に指定する。標準 SSH 連携は入力した `.ssmm` ホスト名を使うため、両方式のホスト鍵の記録は異なる。

ラッパーと生成設定は `ControlPath none` を指定し、接続ごとに proxy を実行する。SSH のユーザー設定で接続先や `ProxyCommand` を変更する場合は、その設定との整合性を確認する必要がある。

> [!NOTE]
> SSH とファイル転送の内容は Session Manager のセッションログに記録されない。監査が必要な場合は接続先で記録する。制約の詳細は、[AWS の SSH 接続設定](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-getting-started-enable-ssh-connections.html) を参照。
