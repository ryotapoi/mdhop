# ストーリー（代表フロー・厳密モード）

このドキュメントは、実際に人間/AIが行う操作の流れをまとめる。
ここで必要になったコマンドとオプションを、外部仕様に反映する。

## 0. 初期ビルドで衝突 → 救済

- 状況: 同名ノートが既に存在し、厳密モードの build が通らない
- 手順:
  1) `mdhop build --vault .`
  2) エラーに表示された衝突名を確認
  3) `mdhop disambiguate --name A --target sub1/A.md --scan`
  4) `mdhop build --vault .`
- 期待結果:
  - build が通り、曖昧リンクが一意化される

## 1. 初回導入 → 解決

- 状況: 新しいVaultでmdhopを使い始める
- 手順:
  1) `mdhop build --vault .`
  2) `mdhop resolve --vault . --from Notes/A.md --link '[[Project]]'`
- 期待結果:
  - `.mdhop/index.sqlite` が生成される
  - `[[Project]]` の解決先が返る（曖昧ならエラー）

## 2. 編集 → 差分更新 → 解決

- 状況: ノートを編集してリンクを追加した
- 手順:
  1) `mdhop update --vault . --file Notes/A.md`
  2) `mdhop resolve --vault . --from Notes/A.md --link '[[NewNote]]'`
- 期待結果:
  - A.md のリンク情報が最新化される
  - `[[NewNote]]` の解決結果が返る

## 3. 新規ノート追加 → 反映 → 解決

- 状況: 新しいノートを追加した
- 手順:
  1) `mdhop add --vault . --file Notes/NewNote.md`
  2) `mdhop resolve --vault . --from Notes/A.md --link '[[NewNote]]'`
- 期待結果:
  - 新規ノートがインデックスに反映される
  - `[[NewNote]]` が解決できる

## 4. Backlinks 起点の関連探索

- 状況: あるノートに関連するノートを辿りたい
- 手順:
  1) `mdhop query --vault . --file Notes/Design.md`
  2) 返ってきた Backlinks の一つに対して `mdhop query --file <that note>`
- 期待結果:
  - Backlinks 経由で関連ノートが辿れる

## 5. タグ起点の関連探索

- 状況: あるタグを持つノートを一覧したい
- 手順:
  1) `mdhop query --vault . --tag '#project'`
- 期待結果:
  - #project タグを持つノート一覧が返る

## 6. 2hop で関連探索

- 状況: 直接リンクが無いが関連の強いノートを探したい
- 手順:
  1) `mdhop query --vault . --file Notes/Design.md`
  2) 返ってきた 2hop のノートを開く
- 期待結果:
  - 2hop による関連ノートが得られる

## 7. 診断 → 曖昧性の検出

- 状況: 同名ノートが増えて曖昧なリンクが出てきた
- 手順:
  1) `mdhop diagnose --vault .`
- 期待結果:
  - basename 衝突の一覧が出る
  - phantom は参考情報として出る

## 8. phantom 解消

- 状況: phantom のリンク先ノートを作成した
- 手順:
  1) `mdhop diagnose --vault .`
  2) phantom の名前でノートを作成する
  3) `mdhop add --vault . --file Notes/Phantom.md`
- 期待結果:
  - phantom が解消される

## 9. 本文タグの取り込み

- 状況: 本文中のタグを関連探索に使いたい
- 手順:
  1) `mdhop build --vault .`
  2) `mdhop query --vault . --file Notes/Tagged.md`
- 期待結果:
  - 本文タグが `Tags` に出る

## 10. frontmatter tags の取り込み

- 状況: frontmatter tags を関連探索に使いたい
- 手順:
  1) `mdhop build --vault .`
  2) `mdhop query --vault . --file Notes/Tagged.md`
- 期待結果:
  - frontmatter tags が `Tags` に出る

## 11. Markdown link の解決

- 状況: `[text](path/to/Note.md)` を解決したい
- 手順:
  1) `mdhop build --vault .`
  2) `mdhop resolve --vault . --from Notes/A.md --link '[text](path/to/Note.md)'`
- 期待結果:
  - Markdown link が解決できる

## 12. code fence / inline code の除外

- 状況: コード内の `[[link]]` や `#tag` を誤検出したくない
- 手順:
  1) `mdhop build --vault .`
  2) `mdhop query --vault . --file Notes/Code.md`
- 期待結果:
  - コード内のリンク/タグは無視される

## 13. fence への移動を反映

- 状況: 既存リンクが code fence に移動して無効化された
- 手順:
  1) `mdhop update --vault . --file Notes/A.md`
  2) `mdhop query --vault . --file Notes/A.md`
- 期待結果:
  - 以前のリンクが消えた状態が反映される

## 14. ファイル移動

- 状況: ノートを別フォルダに移動した
- 手順:
  1) `mdhop move --vault . --from Notes/OldPath.md --to Archive/OldPath.md`
- 期待結果:
  - 旧パスがインデックスから削除される
  - 新パスが登録される
  - 参照側のリンクが必要に応じて書き換わる

## 15. コンテキスト付きクエリ

- 状況: リンク周辺の文脈も含めて関連ノートを確認したい
- 手順:
  1) `mdhop query --vault . --file Notes/Design.md --include-snippet 3`
- 期待結果:
  - Backlinks にリンク周辺の行が含まれる

## 16. 曖昧リンクの解消（disambiguate）

- 状況: basename 衝突が発生し、既存の `[[a]]` を一意化したい
- 手順:
  1) `mdhop disambiguate --name a`
- 期待結果:
  - `[[a]]` が `[[path/to/a]]` へ書き換えられる
  - 候補が複数ある場合は `--target` を要求される

## 17. 削除（delete）

- 状況: ノートを削除した
- 手順:
  1) `mdhop delete --vault . --file Notes/OldPath.md`
- 期待結果:
  - 参照があれば phantom、なければ完全削除

## 18. 統計情報の確認

- 状況: Vault の概要を把握したい
- 手順:
  1) `mdhop stats --vault .`
- 期待結果:
  - ノート数、リンク数などの統計が返る
