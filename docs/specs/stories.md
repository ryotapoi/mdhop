# ストーリー（代表フロー・厳密モード）

このドキュメントは、実際に人間/AIが行う操作の流れをまとめる。
ここで必要になったコマンドとオプションを、外部仕様に反映する。

## 0. 初期ビルドで曖昧リンク → 救済

- 状況: 同名ノートが存在し、かつ曖昧リンクがあるため build が通らない
- 手順:
  1) `mdhop build --vault .`
  2) エラーに表示された衝突名を確認
  3) `mdhop disambiguate --name A --target sub1/A.md --scan`
  4) `mdhop build --vault .`
- 期待結果:
  - build が通り、曖昧リンクが一意化される

## 0.1 同名ノートあり・曖昧リンクなし（build成功）

- 状況: 同名ノートがあるが、曖昧リンクが存在しない
- 手順:
  1) `mdhop build --vault .`
- 期待結果:
  - build が成功する

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
  2) 返ってきた Backlinks の一つに対して `mdhop query --vault . --file Notes/Related.md`
- 期待結果:
  - Backlinks 経由で関連ノートが辿れる

## 5. タグ起点の関連探索

- 状況: あるタグを持つノートを一覧したい
- 手順:
  1) `mdhop query --vault . --tag '#project' --relations backlinks`
- 期待結果:
  - #project タグを持つノートが backlinks に返る。tag 入口の outgoing / twohop は空。

## 6. 2hop で関連探索

- 状況: 直接リンクが無いが関連の強いノートを探したい
- 手順:
  1) `mdhop query --vault . --file Notes/Design.md --relations twohop`
  2) 返ってきた 2hop のノートを開く
- 期待結果:
  - 入口→経由先←対象の関係で、一件の対象に共有する全経由先が付く。タグや索引済み親タグも経由先になる。

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
  - 本文タグが `outgoing` に tag として出る

## 10. frontmatter tags の取り込み

- 状況: frontmatter tags を関連探索に使いたい
- 手順:
  1) `mdhop build --vault .`
  2) `mdhop query --vault . --file Notes/Tagged.md`
- 期待結果:
  - frontmatter tags が `outgoing` に tag として出る

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
  - backlinks はリンク元、outgoing は入口、twohop は対象→可視経由先の本文から根拠行を返す。`--include-head 3` を足すと返却 note の本文冒頭を付ける。

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

## 19. 同期前の索引差分確認

- 状況: 更新や build の前に、索引とディスクのずれだけを確認したい
- 手順:
  1) `mdhop status --vault . --format json`
- 期待結果:
  - 未登録・変更済み・削除済みが返り、索引と vault は変更されない

## 20. frontmatter key に限定した関連探索

- 状況: `sources` key 由来の直接参照だけを確認したい
- 手順:
  1) `mdhop query --vault . --file Notes/Design.md --link-key sources --relations backlinks,outgoing`
- 期待結果:
  - backlinks / outgoing は指定 key 由来に限定される

## 21. frontmatter 診断から修正位置を開く

- 状況: meta-check または meta-validate の finding を修正したい
- 手順:
  1) `mdhop meta-check --vault . --key sources --format json`
  2) 結果の `source_path` と `line` を開く
- 期待結果:
  - finding ごとに source の行位置が得られる

## 22. 入口の属性と本文冒頭を確認

- 状況: 関連探索をせず note 一件の属性を読む
- 手順: `mdhop inspect --file Notes/Design.md --fields tags,meta --include-head 5 --format json`
- 期待結果: 索引の葉タグと metadata、frontmatter と先頭空行を除く本文冒頭を返す。head 未指定では本文を読まない。query の hide / via 条件は適用しない。

## 23. 経由先の選択・非表示とページ取得

- `mdhop query --file Notes/Design.md --relations twohop --via tag:project --limit 20 --offset 20 --format json` で共有タグ経由の関連対象をページ取得する。続きは `page.next_offset` を使う。
- `--hide-tag project` は経由識別子だけを隠し、対象を残す。`--exclude-via-tag project` はその経由から関係を作らない。対象自体の選択は `--path` / `--where` で行う。
- 複数関係のページ指定はエラー。旧 query の移行は [CLI 契約](overview.md#query-の-cli-契約)参照。
