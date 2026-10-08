# 版を切る変更とリリース

## 方針

- バージョンは SemVer に従う。タグだけの版と GitHub Release を作成する版を分け、GitHub Release は明示的に指示されたバージョンだけで作成する。
- 途中のタグだけの版も英語・日本語の CHANGELOG に個別の見出しで記録する。既存のタグは保持し、上書き・付け替えを行わない。
- 指定バージョンのタグが既にある場合は、その commit と CHANGELOG の対応を確認する。同じ版を重ねて切らず、既存タグを使った GitHub Release が必要なら公開手順へ進む。対応が違う場合は新しい版の指定を確認する。

## versioning：版を切る手順

1. 直前のタグと指定バージョンを確認し、その間の commit・diff・backlog から版に含める変更を確定する。未完了のタスクは残す。
2. 対象 diff に基づき、`README.md`・`README.ja.md`、CLI ヘルプ、その他の文書の更新要否を確認し、必要な更新を行う。[外部依存のライセンス通知](licensing.md)に従って、通知を実際の依存・Go version に合わせる。
3. `CHANGELOG.md` と `CHANGELOG.ja.md` の記載漏れを確認し、未記載の各版を個別の見出しで追記する。backlog のタスク文を写さず、対象 diff に基づく最終的なユーザー影響を書く。
4. 完了タスクを消す前に、理由と守るべき制約がコード・テスト・`docs/`・ADR へ移っていることを確認する。今回の版に入る完了タスクを、英語・日本語の CHANGELOG 更新と同じ変更で backlog から削除する。
5. [変更時の検証](verification.md)の必須 gate と変更内容に対応する確認を通し、版を切る変更を commit する。
6. 指定バージョンのタグが存在しないことを再確認して、版を切る commit にタグを作成する。既存の途中の版のタグも保持する。

## release：GitHub Release の公開手順

1. 直前の GitHub Release と指定バージョンを確認し、その間のタグ・commit・CHANGELOG から公開範囲を確定する。最新タグや backlog の指定グループだけを対象にしない。
2. 指定版のタグがなければ versioning を行う。タグがある場合はその commit と CHANGELOG を照合し、版を重ねて切らない。GitHub Release を作成しなかった途中の版も記載済みであることを確認する。
3. 通知ファイルが対象タグに含まれ、配布するビルドの依存・Go version に合っていることを確認する。バイナリの配布物には `LICENSE` と `THIRD-PARTY-NOTICES.txt` を含める。
4. 必要な commit とタグを push し、指定バージョンのタグで `gh release` を作成する。既存のタグと Release を重複作成しない。
5. Release 本文は直前の GitHub Release 以降の主な変更の短い要約と、英語・日本語の CHANGELOG へのリンクとする。リンクは `main` ではなく今回のリリースタグのファイルに固定する。
