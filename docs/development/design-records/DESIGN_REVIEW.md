# ssmm 設計レビュー記録

> [!NOTE]
> 本ドキュメントは実装前の設計レビュー記録である。ここに記録した検証結果は現行実装の検証結果ではない。現行の開発手順と検証範囲は [開発ガイド](../README.md) を参照。

本ドキュメントは、2026-09-19 に行った設計資料の技術レビューの指摘、設計への反映、確認結果を記録する。対象は DESIGN.md、ARCHITECTURE.md、IMPLEMENTATION.md であり、レビュー時点で Go の実装と go.mod は存在しない。

## 指摘と反映

重要度は、接続先・認証・子プロセスの残存に影響する事項を高、設定の意味や実装方法の不一致を中とする。以下は実装で発生した不具合の報告ではなく、設計の矛盾と未定義だった契約の評価である。

指摘と反映先を、以下に示す。

| ID | 重要度 | 指摘と発生条件 | 設計への反映 |
| --- | --- | --- | --- |
| R01 | 高 | OpenSSH の終了時に送られる SIGHUP が後処理の対象に含まれていない。proxy だけが終了し AWS CLI・plugin が残る可能性がある | proxy 専用の子プロセスグループ、SIGHUP の転送、内外の終了猶予を定義 |
| R02 | 高 | SDK の標準設定だけで AssumeRole 全般を扱えるように読めるが、直接の MFA AssumeRole には TokenProvider が必要 | 初版は追加入力のない認証経路に限定し、CLI のキャッシュだけで SDK の対応を判断しない |
| R03 | 中 | DESIGN.md は StartSession の拒否と TargetNotConnected の分類を要求し、ARCHITECTURE.md は外部 CLI の文言を解析しないとしていた | SDK のエラー分類と、外部 CLI の標準エラー出力・終了コードの保持を分離 |
| R04 | 中 | 鍵の「上書き」が、既存 SSH 設定の IdentityFile も置き換えるように読めた | ssmm が渡す 1 件の設定値だけを上書きし、他の鍵は併存すると明記 |
| R05 | 高 | 検索イベントと UI 通知の契約が未定義で、どちらも省略可能な通知として実装する余地があった | ページイベントは省略禁止とし、集約済みスナップショットだけを置き換える |
| R06 | 高 | Name 指定の画面で、フィルタ編集後も自動選択を許すかが未定義だった | 対話でのフィルタ編集後は ManualOnly に固定し、候補 1 件でも Enter を必要とする |
| R07 | 高 | 標準 SSH 設定に HostName と CanonicalizeHostname がなく、後続の汎用設定で %h とホスト鍵の参照名が変わり得た | 管理用設定に HostName %h と CanonicalizeHostname no を追加 |
| R08 | 中 | 保存直前の比較だけで、外部プロセスの編集を上書きしないと断定していた | 専用ファイルのロックで init 同士を直列化し、非協調の外部編集との原子性を保証しないと明記 |
| R09 | 中 | DESIGN.md と ARCHITECTURE.md で config・inventory・openssh の責務が異なり、実装順序も IMPLEMENTATION.md と異なっていた | パッケージ責務と実装順序の定義箇所を一本化し、CODE_STRUCTURE.md にファイルと型を具体化 |
| R10 | 中 | リージョンと ID による探索省略の条件が外形仕様では広く、内部設計では proxy だけに限定されていた | 探索省略は proxy の ID + --region に限定し、通常の接続では EC2 状態を確認 |
| R11 | 中 | パスをシェルで展開しない規則だけでは、IdentityFile 自身による %・${...} の展開を抑止できない | % をエンコードし、${...} を含む鍵パスは初版で拒否。起動引数は -o IdentityFile を使用 |
| R12 | 中 | SFTP を前提としながら、旧 SCP プロトコルを既定とする OpenSSH の扱いが未定義だった | scp は OpenSSH 9.0 以降を必要条件とし、旧プロトコルへの切り替えを提供しない |

動作条件の詳細は、[ARCHITECTURE.md](ARCHITECTURE.md) を参照。型、検索イベント、状態遷移、保存と実行の契約の詳細は、[CODE_STRUCTURE.md](CODE_STRUCTURE.md) を参照。

## 技術的な根拠

外部仕様と内部設計上の推論を区別する。一次資料から確認した挙動を根拠に、ssmm の制約と検証条件を定義した。

### OpenSSH の終了処理

OpenSSH 9.6p1 の接続実装は、終了処理で ProxyCommand の PID に SIGHUP を送り、その終了を待たない。したがって、ssmm proxy が AWS CLI を子として起動する方式では、proxy 自身のシグナル受付と子の終了処理が必要になる。この後半は ssmm のプロセス構造に対する設計上の帰結である。根拠は、[OpenSSH 9.6p1 の sshconnect.c](https://github.com/openssh/openssh-portable/blob/V_9_6_P1/sshconnect.c) を参照。

Go の CommandContext は標準では直接の子を Kill する。AWS CLI が起動する plugin まで終了させる契約の代わりにはならない。根拠は、[os/exec のキャンセル契約](https://pkg.go.dev/os/exec#CommandContext) を参照。

### SSH 設定の優先順位と展開

OpenSSH の多くの設定は最初の値を採用するが、IdentityFile は複数の指定を追加する。HostName の %h と ProxyCommand のトークンはそれぞれの文脈で展開される。固定引数のクォートだけで、設定値の優先順位やパス内のトークン展開まで解決できるとは仮定しない。根拠は、[OpenSSH の設定マニュアル](https://man.openbsd.org/ssh_config.5) と [鍵パスの展開実装](https://github.com/openssh/openssh-portable/blob/V_9_6_P1/ssh.c) を参照。

### AWS の認証と CLI 委譲

SDK は明示プロファイルと環境変数経由の選択を異なる優先順位で処理する。MFA を伴う直接の AssumeRole では TokenProvider を検証する。プロファイル名だけを内部 proxy へ渡して指定元を失う実装は、この認証経路を維持できない。根拠は、[SDK の認証解決](https://github.com/aws/aws-sdk-go-v2/blob/main/config/resolve_credentials.go) と [botocore の認証解決](https://github.com/boto/botocore/blob/develop/botocore/credentials.py) を参照。

AWS CLI v2 は StartSession と plugin の起動を処理する。ssmm が通常の子プロセスとして CLI を起動する方式では、その API エラーを SDK のエラー型として受け取れない。CLI の文言解析を禁止する設計との整合性から、構造化分類の範囲を SDK の呼び出しに限定した。根拠は、[AWS CLI v2 の Session Manager 実装](https://github.com/aws/aws-cli/blob/v2/awscli/customizations/sessionmanager.py) を参照。

### 検索の完全性

EC2 の全ページ取得成功は、複数リージョンの同一時刻の状態を保証しない。取得成功と一時点での一意性を混同せず、選択後は ID を固定する設計を維持した。根拠は、[DescribeInstances の結果整合性](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html) を参照。

SSM に対応行がないことを未管理と断定しない設計も維持した。停止済みノードが返らない API の性質と、途中取得失敗を区別する必要があるためである。根拠は、[DescribeInstanceInformation](https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_DescribeInstanceInformation.html) を参照。

## 実行したローカル検証

検証環境は Linux、OpenSSH_9.6p1 Ubuntu-3ubuntu13.19、OpenSSL 3.0.13 である。一時ディレクトリの設定と記録用 proxy を使い、AWS API、実際の SSH サーバー、個人の ~/.ssh/config は使用していない。

実行した検証と観測結果を、以下に示す。

| 検証 | 方法 | 結果 |
| --- | --- | --- |
| IdentityFile の併存 | ssh -G に既存 IdentityFile と -i の実在パスを指定 | 両方の identityfile が出力された |
| IdentitiesOnly の効果 | ssh -G に明示 IdentityFile と IdentitiesOnly=yes を指定 | 既存 IdentityFile は削除されなかった |
| 汎用 HostName の影響 | Host * の HostName を後続に配置 | 対策なしでは書き換わり、HostName %h の追加後は元のホスト名を保持した |
| Include の状態 | 管理用 Host を Include し、続くグローバル設定を別ホストにも適用 | 元ファイルのグローバル設定は維持された |
| ProxyCommand の固定引数 | 空白、単一・二重引用符、ドル記号、% を含む引数を記録用 proxy へ渡す | sh で元の argv と一致した |
| 鍵パス内の % | ssh -vv と記録用設定で %% を渡す | 展開後のパスにリテラルの %h が残った |
| 鍵指定の引数形式 | 空白と %h を含む実在パスで -i と -o IdentityFile を比較 | -i はエンコード済みパスの存在確認で失敗し、-o は実際のパスへ展開した |
| 鍵パス内の ${...} | SSH 設定で未定義の環境変数を含むパスを指定 | 引用符があっても環境変数展開エラーとなった |
| proxy の終了シグナル | 信号を記録する proxy を起動し、SSH の接続タイムアウトを発生させる | SIGHUP を観測した |

ProxyCommand の引数検証とシグナル検証では SSH サーバーを用意していないため、ssh 自身の終了コード 255 は予定した結果である。検証の成否は記録された argv とシグナルで判定した。

設計資料 5 ファイルのローカルリンク 14 件、アンカー、コードブロックの閉じ忘れ、導入文、行末の空白も確認した。

## 残る検証

設計の修正と実行互換性の証明は分けて扱う。以下の事項は実装段階の完了条件であり、設計レビューだけでは対応済みとしない。

未検証事項と確認する段階を、以下に示す。

| 未検証事項 | 必要な確認 |
| --- | --- |
| SDK と CLI の認証一致 | 明示プロファイル・環境認証・SSO・AssumeRole・credential_process を実 AWS で比較 |
| AWS CLI と plugin の通信 | plugin を含む正常接続、異常出力、終了、AWS 側のセッション状態を確認 |
| プロセスの後処理 | 実装した Runner で入れ子の子、端末移譲、リサイズ、シグナル競合を PTY で確認 |
| OS とシェルの互換性 | macOS、WSL、bash、zsh でクォートと端末の動作を確認 |
| 検索と TUI | 遅延、ページ途中失敗、連続更新、最終通知、古い世代の選択を実装で検証 |
| 保存の耐障害性 | ロック、シンボリックリンク、各保存地点の失敗を注入 |
| Go と依存ライブラリ | リリースを固定し、ビルド、単体テスト、race、vet を実行 |

実 AWS への接続と Go の実装に対するテストは実施していない。設計レビュー時点の環境には Session Manager plugin が見つからず、実 AWS の検証用設定も選択していない。Git の履歴情報を持たないワークスペースのため、変更の確認には編集前ファイルとの比較を使用した。
