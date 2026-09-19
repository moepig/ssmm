# ssmm

ssmm は、EC2 インスタンスを検索し、AWS Systems Manager Session Manager 経由で接続する CLI ツールである。EC2 の Name タグやインスタンス ID で接続先を指定できる。

## 主要な機能

- 複数リージョンの EC2 一覧と SSM 状態の表示
- Name タグ、任意のタグ、部分一致フィルタによる検索と対話選択
- Session Manager のシェル接続
- Session Manager を経由した SSH 接続とファイル転送
- AWS プロファイルごとの検索範囲・SSH 設定と、標準の `ssh`・`scp` との連携
- 一覧の JSON 出力とインスタンス ID 出力

## 使用例

AWS 認証情報と接続先の Session Manager 設定を準備した環境で、次のように実行する。`prod` は AWS プロファイル名、`web-01` は EC2 の Name タグであり、環境に合わせて置き換える。

```sh
ssmm list -p prod
ssmm -p prod
ssmm connect web-01 -p prod
ssmm ssh ec2-user@web-01 -p prod --identity-file ~/.ssh/prod.pem
ssmm scp -p prod --user ec2-user --identity-file ~/.ssh/prod.pem ./report.csv web-01:/tmp/report.csv
ssmm list -p prod --tag Environment=production --output json
```

接続コマンドは操作端末がある場合に選択画面を表示する。`--non-interactive` を指定すると、検索が完了し、候補が 1 件で、EC2 の状態が `running` の場合に接続する。

## 導入

Go 1.24 以降で、リポジトリのルートからビルドする。

```sh
go build -o ssmm ./cmd/ssmm
./ssmm --help
```

以降の操作では、生成した `ssmm` を `PATH` の通ったディレクトリへ配置する。シェル接続には AWS CLI と Session Manager plugin、SSH 接続と転送には追加で OpenSSH が必要である。実装は Unix 環境を前提とする。

導入手順と AWS 側の前提条件の詳細は、[セットアップ](docs/usage/getting-started.md) を参照。

## ドキュメント

- 操作方法、設定、コマンドの詳細は、[利用ガイド](docs/usage/README.md) を参照。
- ビルド、テスト、内部構成、現行仕様の詳細は、[開発ガイド](docs/development/README.md) を参照。
