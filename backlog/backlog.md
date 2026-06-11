# Backlog

## v0.11.0 — frontmatter 検査と search 強化

frontmatter の品質検査と search の強化。v0.10.0 を実 vault に当てた結果（残る phantom、`link_keys` の効き方）を設計判断の材料にする。詳細は [diagnostics.md](diagnostics.md)。

- [x] frontmatter 任意 key の path-like value 検査（`meta-check`）。`--key` + `--kind path|wikilink`。URL / 空値は許可、not_found / ambiguous / vault_escape / not_wikilink を区別報告。meta-validate とは別コマンド（ADR 0019）
- [x] frontmatter schema validation（`meta-validate`: `--require` 必須 key・enum 外・型 parse 不可。空値は index 時に落ちるため missing に吸収。mdhop.yaml の `meta.types` を検査側でも使う。型/enum 違反は index 時の value_type フォールバックで検出）
- [x] search の computed fields（行数・リンク数を `--fields` / `--sort` で）+ `--fields` の meta key 出力対応（ADR 0017）
- [x] `--where` の相対日付比較（`updated<today-90d`。**実施決定**）
- [ ] `changed --since` 変更ファイル列挙（git との差分を見てから要否判断）
- [x] `[[note#見出し]]` の anchor 切れ検出（**実装が軽い場合のみ**） → **実施**。Obsidian の anchor 正規化は句読点除去＋空白畳み込みのみ（kebab-case 不要）で軽量と確認し実装。`diagnose --fields anchors`（opt-in、heading は検査時にディスクから抽出）。ADR 0018
- [x] anchor 検査の設計前に convert.go のパース骨格重複を畳むか判断する → **畳んだ**。本文走査骨格（frontmatter/fence スキップ + stripInlineCode）を `walkBodyLines` に抽出し、parseLinks / parseLinksForConvert を載せ替えた。anchor 検査の heading 走査もこれに載せる（三重化回避）。2026-06-11
- [x] computed fields の設計時に search.go の count / main クエリの WHERE 二重適用を query builder 化するか検討する → **query builder 化は見送り**。computed fields は main クエリの SELECT 句／JOIN にのみ影響し、count クエリは `COUNT(*)` 固定で崩れない。whereSQL は文字列共有のまま維持で十分（YAGNI）。2026-06-11 判断
- [ ] examples skill と README / README.ja を v0.11.0 追加分に同期

## Later

- [ ] Obsidian 互換モード（曖昧リンクを暗黙解決。全コマンドに横断影響あり、要望が出たら再検討）
- [ ] 対話的 disambiguate `--interactive`（人間向け UX 改善。Agent は `--scan` で十分）
- [ ] パース層の package 化の再評価
  - **trigger**: v0.10.0 の `meta.link_keys` 着地後、parse 層の入出力（MetaConfig 注入）が安定した時点
  - **経緯**: internal/core は 30 ファイルで `rules/architecture.md` の分割検討条件（20 ファイル超 + 責務グループ明確）に到達。2026-06-10 の audit + module-boundary 判断で「mutation クラスタは resolveMaps / rewriteEntry / dbExecer の密共有で export 面が大きく未成熟、パース層のみが候補だが link_keys で入出力が変わる直前」として package 分割は見送り、file 境界整理（v0.9.0）のみ実施
- [ ] move 系（move.go / move_dir.go / move_helpers.go 計 1,602 行、18 関数パイプライン）の分割再評価
  - **trigger**: move に次の機能要求が来た時点（v0.10.0 / v0.11.0 では触らない）
  - **経緯**: 凝集はしているが backup / rollback のベストエフォート復元コードが分散（2026-06-10 audit）
