# Backlog

## v0.12.1

- [x] Linux CI で Unicode 正規化系テストが落ちる不具合を修正する
  - **症状**: GitHub Actions の `ubuntu-latest` で `go test ./...` が失敗し、`TestAddNormalizesUnicodePathToExistingPhantom` / `TestAddRejectsUnicodeEquivalentExistingIndexPath` / `TestBuildNormalizesUnicodePathsToNFC` / `TestMetaCheckResolvesNFCValueToNFDPath` / `TestResolveNFCLinkAgainstNFDIndexName` が `Café.md` を見つけられない
  - **再現条件**: Linux filesystem 上で NFC / NFD の異なるファイル名を扱う path 正規化テストを実行する場合。macOS ではローカル `go test ./...` が通るため、OS ごとの filesystem 正規化差で CI のみ失敗する
  - **対処案**: テストデータ作成・path lookup・`NormalizePath` 適用位置を Linux 上でも成り立つ形に見直し、Ubuntu CI と macOS の両方で `go test ./...` を通す

- [ ] `rewriteOutgoingRelativeLink` が `filepath.Rel` の戻り値を `filepath.Clean` せず壊れた相対リンクを生成しうる
  - **症状**: `move_helpers.go:642` 付近で `rel := filepath.Rel(filepath.Dir(to), resolvedTarget)` の結果を `filepath.ToSlash(rel)` するだけで `filepath.Clean` していない。`filepath.Rel("other", ".")` は `".."` ではなく `"../."` を返すため、移動後の相対リンクが `[[../.]]` のような末尾 `/.` 付きで書き換えられるケースがある
  - **再現条件**: from がサブディレクトリにあり vault ルート（`..` で解決される先）を指す相対リンクを持ち、その from を別ディレクトリへ move する場合（検証エージェントが Go で `filepath.Rel("other", ".") == "../."` を実測確認済み）
  - **対処案**: `rel = filepath.ToSlash(filepath.Clean(rel))` にする。`move_helpers.go` の 2 箇所（wikilink / markdown 系）両方を確認。回帰テストを追加する
  - **経緯**: 旧 knowledge.md にあった「`filepath.Rel(dir, ".")` は `"../."` を返す。`filepath.Clean` を適用すること」という教訓。コメント化するとコードの現状（Clean なし）と矛盾するため backlog 化（2026-06-17 の knowledge.md 解体時に分離）

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: v0.10.0 の `meta.link_keys` 着地後、parse 層の入出力（MetaConfig 注入）が安定した時点
  - **経緯**: internal/core は 30 ファイルで `docs/rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施
- [ ] move 系（move.go / move_dir.go / move_helpers.go 計 1,602 行、18 関数パイプライン）の分割再評価
  - **trigger**: move に次の機能要求が来た時点（v0.10.0 / v0.11.0 では触らない）
  - **経緯**: 凝集はしているが backup / rollback のベストエフォート復元コードが分散（2026-06-10 audit）
