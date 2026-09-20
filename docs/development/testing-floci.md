# Floci による検証

本ドキュメントは、Floci の EC2・SSM エンドポイントを使った一覧取得と対話選択の検証手順を説明する。EC2 は Floci の mock mode で登録するため、実際のインスタンスや SSH 接続は起動しない。

## 前提条件

Docker、Docker Compose、AWS CLI、Go と操作端末が必要である。構成は [docker-compose.yml](../../test/floci/docker-compose.yml)、初期データは [01-seed-ec2.sh](../../test/floci/init/ready.d/01-seed-ec2.sh) で定義する。

Floci のエンドポイントは `http://127.0.0.1:4566` である。AWS SDK にはダミーの認証情報を渡す。認証情報は実際の AWS アカウントへ接続するために使用しない。

## 起動と確認

リポジトリのルートで、次のコマンドを実行する。

```sh
./scripts/test-floci-list.sh
```

スクリプトは Floci を起動し、`us-east-1` に `ssmm-floci-demo-a` と `ssmm-floci-demo-b` という Name タグの EC2 インスタンスを 2 台登録する。その後、`ssmm list --output json` を実行し、EC2 の `DescribeInstances` と SSM の `DescribeInstanceInformation` を通した結果を表示する。

続けて `ssmm connect` を起動する。表示された TUI で ↑↓ キーと Enter キーを操作してインスタンスを選択する。キー入力は自動送信しない。選択後に実行する `aws ssm start-session` はダミーの実行ファイルへ置き換えるため、実際の Session Manager 接続は発生しない。ダミーの実行ファイルが受け取ったインスタンス ID が、表示した 2 台のいずれかと一致することを確認する。

SSM Agent は登録しないため、SSM 情報の取得に成功した場合の `ssm_status` は `NotReported` である。この検証は API 応答の変換と対象選択を確認するものであり、SSM Agent、セッション認証、SSH、SFTP の接続試験ではない。

終了時は Compose を停止し、一時ディレクトリ内の設定と実行ファイルを削除する。別の作業で同じ Compose 環境を使用していない状態で実行する。

## 手動起動

Floci を手動で起動する場合は、次のコマンドを実行する。

```sh
docker compose -f test/floci/docker-compose.yml up -d
```

初期データの登録後に一覧を取得する例を、以下に示す。サブシェル内で接続先とダミーの認証情報を限定する。

```sh
(
  unset AWS_PROFILE AWS_DEFAULT_PROFILE
  export AWS_ENDPOINT_URL=http://127.0.0.1:4566
  export AWS_REGION=us-east-1
  export AWS_DEFAULT_REGION=us-east-1
  export AWS_ACCESS_KEY_ID=test
  export AWS_SECRET_ACCESS_KEY=test
  unset AWS_SESSION_TOKEN SSMM_AWS_ENDPOINT_URL
  ssmm list --region us-east-1 --output json
)
```

上記の手動実行では ssmm プロファイルを指定しないため、ssmm 設定ファイルを読み込まない。起動直後は初期データの登録が終わっていない場合があるため、必要に応じて Compose のログを確認する。

## 接続先の指定

ssmm 内の SDK は `SSMM_AWS_ENDPOINT_URL`、`AWS_ENDPOINT_URL` の順に接続先を選び、EC2 と SSM の両方に適用する。`SSMM_AWS_ENDPOINT_URL` は AWS CLI の接続先を変更しない。

テストスクリプトの接続先は `FLOCI_ENDPOINT`、リージョンは `FLOCI_REGION` で変更できる。`FLOCI_ENDPOINT` を変更しても Compose の公開ポート `4566` は変更されない。Compose のイメージは `floci/floci:latest-compat` であり、digest は固定していない。

> [!NOTE]
> スクリプトは既存の `SSMM_AWS_ENDPOINT_URL` を解除しない。設定されている場合、AWS CLI と ssmm の検索先が異なる可能性があるため、実行前に解除する。

## 終了

手動起動した Floci は、次のコマンドで停止する。

```sh
docker compose -f test/floci/docker-compose.yml down
```
