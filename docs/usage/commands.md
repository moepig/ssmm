# コマンド

本ドキュメントは、ssmm のコマンド、検索条件、対話選択、一覧出力、終了コードを説明する。

## コマンド一覧

公開コマンドを、以下にまとめる。`[ ]` は省略可能な引数を示す。

| 形式 | 動作 |
| --- | --- |
| `ssmm [TARGET]` | Session Manager のシェル接続。`connect` と同じ |
| `ssmm connect [TARGET]` | 検索と対象選択の後にシェル接続 |
| `ssmm list` | EC2 と SSM 状態の一覧出力 |
| `ssmm ssh [USER@TARGET]` | 検索と対象選択の後に OpenSSH で接続。対象省略も可能 |
| `ssmm scp [OPTIONS] SRC... DEST` | ローカルと 1 台の EC2 の間でファイル転送。experimental |
| `ssmm proxy TARGET` | OpenSSH の `ProxyCommand` 用の通信ストリーム |
| `ssmm init [SSMM_PROFILE]` | AWS プロファイル名と検索リージョンを保存。`--ssh` 指定時は SSH の既定値も設定 |
| `ssmm ssh-config create` | 選択したプロファイルの標準 SSH 用設定を作成・更新 |
| `ssmm ssh-config delete` | 選択したプロファイルの標準 SSH 用設定を削除 |
| `ssmm completion SHELL` | シェル補完スクリプトを出力 |
| `ssmm help [COMMAND]` | ヘルプを表示 |

各コマンドのフラグは `ssmm COMMAND --help` で確認できる。サブコマンドを使う場合は、その後にフラグを記述する。`-p` は AWS プロファイル、`-r` はリージョン、`-t` は EC2 タグ、`-s` は ssmm プロファイルを指定する。`init` を実行せずに検索・接続できる。

SSH、転送、proxy、`ssh-config` の操作例は [SSH とファイル転送](ssh.md)、`init` の操作と保存対象は [設定](configuration.md) を参照。

## 検索フラグ

コマンドごとの検索フラグを、以下にまとめる。

| フラグ | 動作 | 対応コマンド |
| --- | --- | --- |
| `--profile NAME` / `-p NAME` | AWS プロファイルを選択 | ルート、`connect`、`list`、`ssh`、`scp`、`proxy` |
| `--ssmm-profile NAME` / `-s NAME` | 保存済みの ssmm プロファイルを選択 | ルート、`connect`、`list`、`ssh`、`scp`、`proxy` |
| `--region REGION` / `-r REGION` | 1 リージョンに限定 | ルート、`connect`、`list`、`ssh`、`scp`、`proxy` |
| `--filter TEXT` | 大文字・小文字を区別しない部分一致 | ルート、`connect`、`list`、`ssh` |
| `--tag KEY=VALUE` / `-t KEY=VALUE` | EC2 タグの完全一致。繰り返し指定可能 | ルート、`connect`、`list`、`ssh`、`scp`、`proxy` |
| `--non-interactive` | 対象選択画面を開かず、候補の一意性を要求 | ルート、`connect`、`ssh`、`scp` |

ssmm プロファイルを指定しない場合は設定ファイルを読み込まない。`-s` を指定した場合も、実行時の `-p` と `-r` が保存値より優先する。`init` と `ssh-config` の保存先指定は、[設定](configuration.md) を参照。

初期設定なしで AWS プロファイル、リージョン、タグを指定する例を、以下に示す。

```sh
ssmm -p company-prod -r ap-northeast-1 -t Environment=production
ssmm list -p company-prod -r ap-northeast-1 -t Service=web
```

`list` も `--non-interactive` を受け付けるが、指定の有無によらず選択画面は開かない。`proxy` も常に非対話である。

### TARGET とタグ

TARGET は Name またはインスタンス ID である。`i-` に続く 8 桁または 17 桁の小文字 16 進数を ID と判定する。それ以外の値は Name として扱う。Name は大文字・小文字を区別する完全一致であり、`*` と `?` も文字どおりに照合する。

`--tag` はキーと値の両方を大文字・小文字を区別して完全一致で照合する。複数のタグは AND 条件であり、Name と併用できる。インスタンス ID と `--tag` は併用できない。

Name とタグを組み合わせる例を、以下に示す。

```sh
ssmm connect web-01 -p company-prod --tag Environment=production
ssmm list -p company-prod --tag Environment=production --tag Service=web
```

### 部分一致フィルタ

`--filter` と対話画面の入力は、Name、リージョン、インスタンス ID、プライベート IP、Availability Zone、タグのキーと値を対象とする。空白区切りの各語を AND 条件で照合し、各語はいずれかの項目に含まれていればよい。EC2 状態と SSM 状態は部分一致の対象外である。

部分一致を使う例を、以下に示す。

```sh
ssmm list -p company-prod --filter 'web ap-northeast-1'
ssmm connect -p company-prod --filter web
```

## 対象選択

TARGET を指定した場合、ルート、`connect`、`ssh`、`scp` は検索完了後に候補が 1 件なら接続し、0 件または複数件ならエラーになる。操作端末が使える場合も選択画面を開かない。

TARGET を省略して操作端末が使える場合、ルート、`connect`、`ssh` は検索結果を順次表示する。候補が 1 件でも Enter キーで選択する。停止中などのインスタンスも表示するが、選択できるのは `running` の行だけである。

対話画面の操作を、以下にまとめる。

| キー | 動作 |
| --- | --- |
| 文字入力 | 部分一致フィルタへ追加 |
| Backspace | フィルタの末尾を削除 |
| ↑ / ↓ | 選択位置を移動 |
| Enter | `running` の行を選択して接続 |
| Ctrl+R | 検索をやり直す。フィルタと選択対象を引き継ぐ |
| Esc / Ctrl+C | キャンセル |

選択対象は行番号ではなく、リージョンとインスタンス ID の組で保持する。検索結果の先頭へ別の行が追加されても、同じ対象が選択状態に残る。検索中の空のスナップショットを受信しても、保持している選択対象を消去しない。

選択対象が現在の候補にない場合は表示マーカーを消す。対象が再び候補へ現れるまで Enter は接続を確定しない。対象が消失した状態で別の対象へ移るには、↑、↓、またはフィルタの編集を行う。

検索途中でも取得済みの行を手動選択できる。検索全体の成功を確認してから対象を決める必要がある場合は、`--non-interactive` を使う。

TARGET または `--non-interactive` を指定した場合、または `/dev/tty` を開けない場合は、全検索範囲の EC2 取得成功と検索終了を待つ。候補が 1 件で `running` の場合だけ接続し、0 件、複数件、不完全な検索結果では失敗する。停止中の行も候補数に含める。

一意な対象へ選択画面を開かずに接続する例を、以下に示す。

```sh
ssmm connect web-01 -p company-prod --region ap-northeast-1
```

`--non-interactive` が無効にするのは ssmm の対象選択である。SSH のホスト鍵確認、鍵のパスフレーズなど、外部コマンドの入力は別に発生し得る。

> [!NOTE]
> 標準出力をリダイレクトしても、操作端末が使える場合は選択画面を開く。自動処理では `--non-interactive` を明示する。

## 一覧出力

`list` は TARGET 引数を受け付けない。Name を絞り込む場合は `--tag Name=VALUE` または `--filter` を使う。出力順は Name、リージョン、インスタンス ID の昇順である。`terminated` のインスタンスは含めない。

出力形式を、以下にまとめる。

| `--output` | 標準出力 |
| --- | --- |
| `table` または省略 | ssmm プロファイル（未指定時は `(none)`）、検索範囲、取得状況、および `NAME`・`REGION`・`INSTANCE ID`・`STATE`・`SSM` の表 |
| `json` | インスタンスの JSON 配列 |
| `id` | 1 行に 1 つのインスタンス ID |

形式を切り替える例を、以下に示す。

```sh
ssmm list -p company-prod --tag Name=web-01
ssmm list -p company-prod --output json
ssmm list -p company-prod --region ap-northeast-1 --output id
```

### JSON のフィールド

配列の各要素が持つフィールドを、以下にまとめる。

| フィールド | 型 | 内容 |
| --- | --- | --- |
| `instance_id` | string | インスタンス ID |
| `name` | string / null | Name。タグがない場合は `null` |
| `region` | string | リージョン |
| `ec2_state` | string | EC2 の状態 |
| `ssm_status` | string | SSM の状態 |
| `private_ip` | string / null | プライベート IP |
| `availability_zone` | string / null | Availability Zone |
| `tags` | object | タグのキーと値。タグがなければ `{}` |

該当するインスタンスがない場合、JSON は `[]`、ID 形式は空出力となり、どちらも成功する。ID 形式にはリージョンを含めないため、リージョンを保持する処理には JSON を使う。

### SSM 状態

SSM 状態の意味を、以下にまとめる。

| 状態 | 意味 |
| --- | --- |
| `Online` | SSM API が `Online` と報告した |
| `ConnectionLost` | SSM API が `ConnectionLost` または `Inactive` と報告した |
| `NotReported` | SSM 情報の全ページ取得に成功したが、該当 ID がなかった |
| `Unknown` | SSM 情報が未確定、取得失敗、または上記に分類できない状態 |

`NotReported` は SSM 未管理であることを断定する値ではない。SSM 状態は接続先の選択を制限しないため、`running` であればどの SSM 状態でも接続を試みる。

### 取得失敗時の出力

いずれかのリージョンで EC2 の取得が失敗した場合、`list` は標準出力へ一覧を出さず、標準エラー出力へ診断を出して終了コード 1 を返す。SSM の取得だけが失敗した場合は、そのリージョンの SSM 状態を `Unknown` として一覧を出力し、標準エラー出力に診断を出す。EC2 取得と出力が成功していれば終了コードは 0 である。

## 終了コード

終了コードの扱いを、以下にまとめる。

| コード | 意味 |
| --- | --- |
| `0` | 成功 |
| `1` | ssmm の一般的な失敗。検索失敗、候補なし、曖昧な対象、依存コマンド不足など |
| `2` | ssmm が入力エラーとして分類した失敗。タグ形式、ユーザー指定の矛盾、初期設定の入力検証など |
| `130` | 対話選択または初期設定のキャンセル |
| 外部コマンドの終了コード | 起動した AWS CLI、`ssh`、`scp` の非 0 終了コードを引き継ぐ |

現行実装では、Cobra による未知のフラグや引数個数のエラーなど、入力に起因しても 1 になる場合がある。エラー分類を終了コード 2 だけで判定してはいけない。Unix で外部コマンドがシグナル終了した場合は `128 + シグナル番号` を返す。
