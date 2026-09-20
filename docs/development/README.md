# 開発ガイド

本ドキュメントは、ssmm のビルドとテスト、リリース手順、内部構成と仕様の参照先、設計記録の位置付けを説明する。

## 開発環境

Go の最低バージョンは [go.mod](../../go.mod) の `go 1.24` である。依存バージョンは `go.mod` と `go.sum` で管理する。設定ロックと端末処理は Unix 専用であり、Windows ネイティブ向けのビルドは対象外である。

リポジトリのルートで、次のコマンドを実行する。

```sh
go build -o ssmm ./cmd/ssmm
go test ./...
```

VCS 情報のないソースコピーで `.git` の状態取得に失敗する場合は、ビルドに `-buildvcs=false` を付ける。

並行処理を変更する場合は、race detector でも確認する。

```sh
go test -race ./...
```

単体テストは実 AWS の認証情報を前提としない。プロセス管理のテストではローカルの子プロセスを起動する。Linux では疑似端末を使うテストもある。

## リリース

`v*` タグを push すると、GitHub Actions で GoReleaser v2 を実行する。`go test ./...` の成功後、Linux と macOS の amd64・arm64 向けバイナリをビルドし、GitHub Releases に公開する。プレリリースのバージョンは GitHub Releases でもプレリリースとして扱う。

配布物は `ssmm_<version>_<os>_<arch>` という名前のバイナリと SHA-256 の `checksums.txt` である。macOS の OS 名は `darwin` となる。バイナリにはリリースバージョンを埋め込み、`ssmm --version` で表示する。公開には workflow の `GITHUB_TOKEN` を使う。

GoReleaser v2 を用意し、リポジトリのルートで次のコマンドを実行すると、設定と公開前のビルドを確認できる。snapshot は `dist/` に生成され、GitHub Releases には公開されない。

```sh
goreleaser check
goreleaser release --snapshot --clean --parallelism 1
```

リリース対象のコミットに SemVer のタグを付け、そのタグを push する。`v0.1.0` を公開する場合のコマンドを、以下に示す。

```sh
git tag v0.1.0
git push origin v0.1.0
```

設定の詳細は、[GoReleaser 設定](../../.goreleaser.yaml) と [Release workflow](../../.github/workflows/release.yaml) を参照。

## ドキュメント構成

現行実装を説明する資料を、以下にまとめる。

| 資料 | 内容 |
| --- | --- |
| [アーキテクチャ](architecture.md) | パッケージの責務、依存関係、実行経路 |
| [仕様](specification.md) | 検索、認証、対象解決、外部実行、設定保存の実装上の契約 |
| [Floci による検証](testing-floci.md) | ローカル API エミュレータでの一覧取得と対話選択 |
| [利用ガイド](../usage/README.md) | 公開コマンド、設定項目、導入と操作手順 |

公開インターフェースの詳細は利用ガイドに集約し、開発資料は内部の状態管理と実装上の制約を扱う。

## テストと検証範囲

既存テストの主な対象を、以下にまとめる。

| 対象 | テストのあるパッケージ |
| --- | --- |
| ID 判定、完全一致と部分一致 | `internal/target` |
| AWS プロファイルとリージョンの優先順位 | `internal/awsconfig`、`internal/app` |
| EC2 ページネーションと繰り返しトークン | `internal/awsapi` |
| 検索結果の逐次通知、EC2 と SSM の失敗の区別 | `internal/inventory` |
| 一覧出力、端末表示、検索の更新操作 | `internal/cli`、`internal/render`、`internal/tui` |
| TOML、相対パス、設定保存、部分的な保存失敗 | `internal/config`、`internal/cli` |
| SSH・SCP の引数、引用符、proxy のパラメータ | `internal/openssh`、`internal/session` |
| SSH 設定生成と Include | `internal/sshconfig` |
| 終了コード、シグナル、環境変数、端末の制御 | `internal/process` |

テスト成功だけで、実 AWS 上の認証経路や Session Manager 接続を確認したことにはならない。Floci のスクリプトも接続部分をダミーの実行ファイルへ置換する。

接続やプロセス管理を変更した場合は、実行対象環境で AWS CLI、Session Manager plugin、OpenSSH、SSM Agent のバージョンを記録し、シェル接続、SSH、転送、キャンセル後の子プロセス終了を確認する。認証処理を変更した場合は、SDK と CLI の両方が同じ意図の認証経路を使用することを確認する。

## 設計記録

`design-records/` は実装前の検討とレビューを保存する。記載された仕様、ファイル構成、検証結果は現行実装の保証ではない。

保存した資料を、以下にまとめる。

| 記録 | 内容 |
| --- | --- |
| [DESIGN.md](design-records/DESIGN.md) | 外形仕様の設計案 |
| [ARCHITECTURE.md](design-records/ARCHITECTURE.md) | 実装前のアーキテクチャ設計 |
| [CODE_STRUCTURE.md](design-records/CODE_STRUCTURE.md) | 型、インターフェース、配置の詳細設計 |
| [IMPLEMENTATION.md](design-records/IMPLEMENTATION.md) | 段階的な実装計画 |
| [DESIGN_REVIEW.md](design-records/DESIGN_REVIEW.md) | 実装前の設計レビューと検証記録 |

実装と設計案の主な差異は、[仕様の設計案との差異](specification.md#設計案との差異) を参照。
