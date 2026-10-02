# リリース

## タグと GitHub Release

- タグを打つだけのバージョンと、GitHub Release を作成するバージョンを分ける。タグごとに GitHub Release を作成する必要はない。
- GitHub Release は明示的に指示されたバージョンで作成する。途中のバージョンは CHANGELOG に記録し、GitHub Release は作成しない。

## GitHub Release の手順

1. 直前の GitHub Release と今回指定されたバージョンを確認し、その間のタグ・commit・backlog から対象範囲を確定する。最新のタグや backlog の指定バージョンのグループだけを対象範囲としない。
2. 対象範囲の commit の diff を確認し、`README.md`・`README.ja.md`、CLI ヘルプ、その他の文書の更新要否を確認して、現在の実装・仕様に合わせて必要な更新を行う。
3. `CHANGELOG.md` と `CHANGELOG.ja.md` に、未記載の各バージョンを個別の見出しで追記する。GitHub Release を作成しなかった途中のバージョンも含め、今回のバージョンだけにまとめない。既存の記載も対象範囲に漏れがないか確認する。backlog のタスク文をそのまま写さず、対象 commit の diff に基づく最終的なユーザー影響を書く。
4. 英語・日本語の CHANGELOG への反映と同じコミットで、記載した変更に対応する完了済みのバージョングループを backlog から削除する。未完了のタスクは残す。
5. [verification](verification.md) の必須 gate と変更内容に対応する確認を通し、必要な更新をコミットする。指定バージョンのタグとリリース commit の対応を確認し、タグがなければ作成して、必要な commit とタグを push する。既存の途中のバージョンのタグは保持する。
6. 指定バージョンのタグで `gh release` を作成する。本文は、直前の GitHub Release 以降の主な変更の短い要約と、英語・日本語の CHANGELOG へのリンクとする。リンク先は `main` ではなく、今回のリリースタグのファイルに固定する。
