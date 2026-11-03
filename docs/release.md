# リリース

版を切る・GitHub Release を公開する共通の手順は `release` skill に従う。本書はこのプロジェクト固有の事項だけを定める。

## 対象

- タグだけの版と、GitHub Release を作成する版を分ける。GitHub Release は明示的に指示された版だけで作成する。
- 途中のタグだけの版も、`CHANGELOG.md` と `CHANGELOG.ja.md` に個別の見出しで記録する。

## 版を切るときに同梱する更新

- 対象の差分に基づき、`README.md`・`README.ja.md`、CLI ヘルプ、その他の文書の更新要否を確認し、必要な更新を行う。
- [外部依存のライセンス通知](licensing.md)に従って、通知を実際の依存・Go version に合わせる。
- `CHANGELOG.md` と `CHANGELOG.ja.md` に、未記載の各版を個別の見出しで追記する。
- [変更時の検証](verification.md)の必須 gate と、変更内容に対応する確認を通す。

## 公開

- 通知ファイルが対象タグに含まれ、配布するビルドの依存・Go version に合っていることを確認する。バイナリの配布物には `LICENSE` と `THIRD-PARTY-NOTICES.txt` を含める。
- 必要な commit とタグを push し、指定版のタグで `gh release` を作成する。
- Release 本文は、直前の GitHub Release 以降の主な変更の短い要約と、英語・日本語の CHANGELOG へのリンクとする。リンクは `main` ではなく今回のリリースタグのファイルに固定する。
