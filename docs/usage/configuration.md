# 設定

本ドキュメントは、AWS プロファイルと検索範囲の選択、ssmm の設定ファイル、`init` による更新を説明する。

## AWS プロファイル

通常のコマンドでは、`--profile` / `-p`、`AWS_PROFILE`、`default` の順にプロファイル名を決定する。`NAME.PROFILE.ssmm` 形式のホスト名では、ホスト名の `PROFILE` を使う。ホスト名と `--profile` の指定が異なる場合はエラーになる。

`AWS_DEFAULT_PROFILE` はプロファイル選択に使わない。`--profile`、ホスト名、`AWS_PROFILE` による指定では、AWS の共有設定または認証情報ファイルにそのプロファイルが存在する必要がある。無指定の `default` では、環境変数やロールによる認証情報の取得も使用でき、`default` の共有設定は必須ではない。

ssmm の設定は AWS のプロファイルと同じ名前で保存する。ssmm の設定だけを作成しても AWS のプロファイルは作成されない。

名前を指定する例を、以下に示す。

```sh
ssmm list -p prod
AWS_PROFILE=prod ssmm list
```

通常の接続で使用できるプロファイル名は、先頭が英数字、以降が英数字・`_`・`.`・`@`・`+`・`-` である。標準 SSH 連携には、さらに小文字英数字とハイフンからなる 1〜63 文字の名前を使い、先頭と末尾をハイフンにしてはいけない。

## 検索リージョン

検索範囲の優先順位を、以下にまとめる。

| 優先順位 | 指定 | 検索範囲 |
| --- | --- | --- |
| 1 | `--region REGION` | 指定した 1 リージョン |
| 2 | `profiles.PROFILE.regions` | 保存したリージョンの一覧 |
| 3 | 指定なし | アカウントで有効な全リージョン |

明示したリージョンも、有効なリージョンの一覧と照合する。無効なリージョンを含む場合は検索を開始しない。`regions = []` はエラーであり、全リージョンを意味しない。全リージョンを使う場合は `regions` を省略する。

AWS の共有設定や環境変数のリージョンは、検索範囲を制限しない。検索範囲が未指定の場合、これらのリージョンはリージョン一覧を取得する API の接続先に使われる。API の接続先も未指定の場合は `us-east-1` を使う。

## 設定ファイル

設定は `~/.config/ssmm/config.toml` に保存する。`XDG_CONFIG_HOME` によるパス変更には対応していない。ファイルがない場合は空の設定として扱う。

設定例を、以下に示す。

```toml
[profiles.prod]
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
| `profiles.PROFILE.regions` | 空でない文字列配列 | 有効な全リージョンを検索する |
| `profiles.PROFILE.ssh.user` | 文字列 | ssmm から SSH ユーザーを指定しない |
| `profiles.PROFILE.ssh.identity_file` | 文字列 | ssmm から鍵ファイルを指定しない |
| `profiles.PROFILE.ssh.port` | 整数 | ラッパーと `proxy` は 22 を使う。`0` も未指定として扱う |
| `profiles.PROFILE.ssh.integration` | 真偽値 | `false`。標準 SSH 連携を生成しない |

`port` の明示値は 1〜65535 である。未知のキー、型の不一致、TOML の構文エラーは拒否する。

### 鍵ファイルのパス

`identity_file` の `~` と `~/` はホームディレクトリに展開する。設定ファイル内の相対パスは `~/.config/ssmm/` を基準とする。`${...}` 形式の環境変数展開はサポートしない。

コマンドラインの `--identity-file` と `init` の対話入力では、相対パスを実行時のカレントディレクトリから解決する。`init` は解決した絶対パスを保存する。

SSH の指定値の優先順位は、[SSH とファイル転送](ssh.md#ssh-の設定値) を参照。

## 初期設定と更新

`init` は選択したプロファイルの設定を作成または更新する。設定用フラグを 1 つも指定せず、操作端末が使える場合は対話入力を開始する。

リージョンと SSH の既定値を保存する例を、以下に示す。

```sh
ssmm init -p prod --regions ap-northeast-1,us-east-1
ssmm init -p prod --user ec2-user --identity-file ~/.ssh/prod.pem --port 22
ssmm init -p prod --all-regions
```

設定用フラグの一覧を、以下にまとめる。

| フラグ | 意味 |
| --- | --- |
| `--regions REGION,...` | 検索リージョンを保存する |
| `--all-regions` | 保存したリージョン制限を解除する |
| `--user USER` | SSH ユーザーの既定値を保存する |
| `--identity-file PATH` | SSH 鍵ファイルの既定値を保存する |
| `--port PORT` | SSH ポートの既定値を保存する |
| `--integration` / `--integration=false` | 標準 SSH 連携を有効化・無効化する |

`--all-regions` と `--regions` は併用しない。フラグによる更新では、指定していない設定を維持する。`init` には `--non-interactive` はなく、自動処理では更新する設定用フラグを明示する。

対話入力では、リージョン欄を空にすると全リージョンへ変更する。SSH ユーザー、鍵、ポート、SSH 連携の各欄を空にすると既存値を維持する。`cancel` または Esc を入力して Enter を押すか、Ctrl+C で保存せずに終了する。

### 更新するファイル

`init` が保存するファイルを、以下にまとめる。

| パス | 内容 |
| --- | --- |
| `~/.config/ssmm/config.toml` | 全プロファイルの設定 |
| `~/.ssh/ssmm/config` | SSH 連携が有効なプロファイルから生成した SSH 設定 |
| `~/.ssh/config` | 生成ファイルを読む `Include` の追加・削除 |
| `~/.config/ssmm/init.lock` | 更新時の排他制御用ファイル |

SSH 連携を無効にした状態でも、`init` は先頭の 3 ファイルを保存する。全プロファイルで SSH 連携が無効になると、管理対象の `Include` を削除する。生成された SSH 設定は手動編集せず、`init` で更新する。

設定ファイルを直接編集した場合、標準 SSH 連携への反映には `init` の再実行が必要である。`ssmm` の実行ファイルを移動した場合も、生成した `ProxyCommand` のパスを更新するために再実行する。

> [!NOTE]
> `init` は設定ファイルを再生成するため、TOML のコメントや元の整形は維持しない。ファイルごとの保存結果を出力し、途中で失敗した場合は保存済みファイルを巻き戻さない。
