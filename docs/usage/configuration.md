# 設定

本ドキュメントは、ssmm・AWS プロファイルと検索範囲の選択、ssmm の設定ファイル、`init` と `ssh-config` による更新を説明する。

## ssmm プロファイルと AWS プロファイル

ssmm プロファイルは、検索リージョン、AWS プロファイル名、SSH の既定値をまとめた設定である。`--ssmm-profile` / `-s` で選択する。実行時に未指定の場合は ssmm 設定を読み込まず、コマンドラインと AWS の認証設定だけを使う。`NAME.PROFILE.ssmm` 形式のホスト名では、ホスト名の `PROFILE` を使う。ホスト名と `--ssmm-profile` の指定が異なる場合はエラーになる。

AWS プロファイルは認証情報を取得するための AWS 共有設定の名前である。ssmm プロファイルの `aws_profile` に指定する。複数の ssmm プロファイルから同じ AWS プロファイルを使用できる。ssmm プロファイル名から同名の AWS プロファイルを自動選択することはない。

AWS プロファイルの選択順序を、以下にまとめる。

| 優先順位 | 指定 | 動作 |
| --- | --- | --- |
| 1 | `--profile` / `-p` | 実行時に指定した AWS 共有設定を使用する |
| 2 | 選択した ssmm プロファイルの `aws_profile` | 保存した AWS 共有設定を使用する |
| 3 | `AWS_PROFILE` | 環境変数で指定した AWS 共有設定を使用する |
| 4 | 指定なし | 環境変数、共有設定の `default`、ロールなど、通常の認証情報取得を使用する |

`AWS_PROFILE` は ssmm プロファイルを選択しない。`AWS_DEFAULT_PROFILE` は使用しない。フラグ、設定、`AWS_PROFILE` で指定した AWS プロファイルは、AWS の共有設定または認証情報ファイルに存在する必要がある。指定なしの場合は `default` の共有設定を必須としない。

ssmm の `prod` に AWS の `company-prod` を割り当てる例を、以下に示す。

```sh
ssmm init -s prod -p company-prod -r ap-northeast-1
ssmm list -s prod
```

ssmm プロファイルを指定しない場合や `aws_profile` が未指定の場合は、実行時の環境変数から AWS プロファイルを選択できる。ssmm プロファイルを指定せず、AWS の `company-prod` を使う例を、以下に示す。

```sh
AWS_PROFILE=company-prod ssmm list
```

実行時に `-s` で指定した ssmm プロファイルが存在しない場合はエラーになる。保存した `default` を使用する場合も `-s default` を明示する。実行時の `-p` と `-r` は選択した ssmm プロファイルの保存値より優先する。

ssmm の設定を作成しても、AWS プロファイルや認証情報は作成されない。

通常の接続で使用できる ssmm プロファイル名は、先頭が英数字、以降が英数字・`_`・`.`・`@`・`+`・`-` である。標準 SSH 連携には、さらに小文字英数字とハイフンからなる 1〜63 文字の名前を使い、先頭と末尾をハイフンにしてはいけない。この追加制限は、参照する AWS プロファイル名には適用しない。

設定ファイルに保存する ssmm プロファイル名と、標準 SSH 連携で使用する SSH ラベルは別の制約で検証する。`Prod_Profile` は通常の検索・接続と設定保存で使用できるが、標準 SSH 連携では使用できない。

既存の設定ファイルに `prod!` などの無効な ssmm プロファイル名がある場合、設定ファイル全体の読み込みが失敗する。TOML の `[profiles.旧名]` と、その配下のテーブル名を有効な名前へ変更してからコマンドを実行すること。ssmm は名前を自動変更しない。

## 検索リージョン

検索範囲の優先順位を、以下にまとめる。

| 優先順位 | 指定 | 検索範囲 |
| --- | --- | --- |
| 1 | `--region REGION` / `-r REGION` | 指定した 1 リージョン |
| 2 | `profiles.PROFILE.regions` | 保存したリージョンの一覧 |
| 3 | 指定なし | アカウントで有効な全リージョン |

明示したリージョンも、有効なリージョンの一覧と照合する。無効なリージョンを含む場合は検索を開始しない。`regions = []` はエラーであり、全リージョンを意味しない。全リージョンを使う場合は `regions` を省略する。

AWS の共有設定や環境変数のリージョンは、検索範囲を制限しない。検索範囲が未指定の場合、これらのリージョンはリージョン一覧を取得する API の接続先に使われる。API の接続先も未指定の場合は `us-east-1` を使う。

## 設定ファイル

設定は `~/.config/ssmm/config.toml` に保存する。`XDG_CONFIG_HOME` によるパス変更には対応していない。検索・接続では、ssmm プロファイルを指定した場合だけ読み込む。指定したプロファイルがない場合はエラーになる。`init` と `ssh-config` はファイルがない場合に新規作成する。

設定例を、以下に示す。

```toml
[profiles.prod]
aws_profile = "company-prod"
regions = ["ap-northeast-1", "us-east-1"]

[profiles.prod.ssh]
user = "ec2-user"
identity_file = "~/.ssh/prod.pem"
port = 22
integration = false
```

各設定項目の意味を、以下にまとめる。

| キー | 型 | 省略時の動作 |
| --- | --- | --- |
| `profiles.PROFILE.aws_profile` | 文字列 | `AWS_PROFILE`、通常の認証情報取得の順に使用する。空文字列も未指定として扱う |
| `profiles.PROFILE.regions` | 空でない文字列配列 | 有効な全リージョンを検索する |
| `profiles.PROFILE.ssh.user` | 文字列 | ssmm から SSH ユーザーを指定しない |
| `profiles.PROFILE.ssh.identity_file` | 文字列 | ssmm から鍵ファイルを指定しない |
| `profiles.PROFILE.ssh.port` | 整数 | ラッパーと `proxy` は 22 を使う。`0` も未指定として扱う |
| `profiles.PROFILE.ssh.integration` | 真偽値 | `false`。標準 SSH 連携を生成しない |

`port` の明示値は 1〜65535 である。未知のキー、型の不一致、TOML の構文エラーは拒否する。

SSH ユーザー名は、制御文字、空白類、先頭の `-`、`@`、`:`、`/`、バックスラッシュを含めない。鍵ファイルのパスは制御文字と `${` を含めない。通常の空白、引用符、バックスラッシュ、`#`、`%` は OpenSSH 設定用に引用して保持する。ポートの `0` は未指定であり、その他は 1〜65535 である。

### 鍵ファイルのパス

`identity_file` の `~` と `~/` はホームディレクトリに展開する。設定ファイル内の相対パスは `~/.config/ssmm/` を基準とする。`${...}` 形式の環境変数展開はサポートしない。

コマンドラインの `--identity-file` と `init` の対話入力では、相対パスを実行時のカレントディレクトリから解決する。`init` は解決した絶対パスを保存する。

SSH の指定値の優先順位は、[SSH とファイル転送](ssh.md#ssh-の設定値) を参照。

## 初期設定と更新

`init` は選択した ssmm プロファイルの AWS プロファイル名と検索リージョンを作成または更新する。保存先は `-s` で指定し、省略時は `default` とする。`--ssh` を指定した場合に限り、SSH ユーザー、鍵ファイル、ポートも設定できる。設定値を渡すフラグを指定せず、操作端末が使える場合は対話入力を開始する。`--ssh` だけを指定した場合も対話入力を開始する。

AWS プロファイル名、リージョン、SSH の既定値を保存する例を、以下に示す。

```sh
ssmm init -s prod --profile company-prod --regions ap-northeast-1,us-east-1
ssmm init --ssh -s prod --user ec2-user --identity-file ~/.ssh/prod.pem --port 22
ssmm init -s prod --all-regions
```

設定用フラグの一覧を、以下にまとめる。

| フラグ | 意味 |
| --- | --- |
| `--ssmm-profile NAME` / `-s NAME` | 保存先の ssmm プロファイルを選択する。省略時は `default` |
| `--profile NAME` / `-p NAME` | 使用する AWS プロファイル名を保存する。空文字列で指定を解除する |
| `--region REGION` / `-r REGION` | 単一の検索リージョンを保存する |
| `--ssh` | SSH 用の設定フラグと対話入力を有効にする |
| `--regions REGION,...` | 検索リージョンを保存する |
| `--all-regions` | 保存したリージョン制限を解除する |
| `--user USER` | SSH ユーザーの既定値を保存する。`--ssh` が必要 |
| `--identity-file PATH` | SSH 鍵ファイルの既定値を保存する。`--ssh` が必要 |
| `--port PORT` | SSH ポートの既定値を保存する。`--ssh` が必要 |

`--region`、`--regions`、`--all-regions` は併用しない。フラグによる更新では、指定していない設定を維持する。`init` には `--non-interactive` はなく、自動処理では更新する設定用フラグを明示する。

対話入力では、リージョン欄を空にすると全リージョンへ変更する。AWS プロファイル、SSH ユーザー、鍵、ポートの各欄を空にすると既存値を維持する。SSH の 3 項目は `--ssh` を指定した場合だけ表示する。`cancel` または Esc を入力して Enter を押すか、Ctrl+C で保存せずに終了する。

## 標準 SSH 設定の管理

`ssh-config create` は選択したプロファイルの `ssh.integration` を `true` にして標準 SSH 用の設定を生成する。`ssh-config delete` は `false` にして対象プロファイルの生成設定を削除する。どちらも検索リージョンと SSH の既定値を保持する。

`init` と `init --ssh` は `ssh.integration` の値を維持し、標準 SSH 用のファイルは更新しない。SSH の既定値を変更した場合は `ssh-config create` を再実行して生成設定へ反映する。

作成・削除の操作例と標準コマンドからの接続方法は、[SSH とファイル転送の標準 SSH 連携](ssh.md#標準-ssh-連携) を参照。

## 更新するファイル

`init` と `ssh-config create` / `delete` が保存するファイルを、以下にまとめる。

| パス | 内容 | 更新するコマンド |
| --- | --- | --- |
| `~/.config/ssmm/config.toml` | 全 ssmm プロファイルの設定 | `init`、`ssh-config create` / `delete` |
| `~/.ssh/ssmm/config` | SSH 連携が有効なプロファイルから生成した SSH 設定 | `ssh-config create` / `delete` |
| `~/.ssh/config` | 生成ファイルを読む `Include` の追加・削除 | `ssh-config create` / `delete` |
| `~/.config/ssmm/init.lock` | 更新時の排他制御用ファイル | `init`、`ssh-config create` / `delete` |

`init` は `--ssh` の有無にかかわらず `~/.ssh/` 以下を読み書きしない。SSH 連携が有効なプロファイルがある場合も同様である。全プロファイルで SSH 連携が無効になると、`ssh-config delete` は管理対象の `Include` を削除する。生成された SSH 設定は手動編集せず、`ssh-config create` で更新する。

設定ファイルを直接編集した場合、標準 SSH 連携への反映には対象プロファイルで `ssh-config create` を再実行する。`ssmm` の実行ファイルを移動した場合も、生成した `ProxyCommand` のパスを更新するために再実行する。

> [!NOTE]
> `init` と `ssh-config create` / `delete` は設定ファイルを再生成するため、TOML のコメントや元の整形は維持しない。ファイルごとの保存結果を出力し、途中で失敗した場合は保存済みファイルを巻き戻さない。
