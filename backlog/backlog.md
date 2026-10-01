# Backlog

## 運用ルール

- Backlog には、今後着手でき、完了を判定できる作業を `- [ ]` 形式のタスクとして置く。
- 対応要否を決める調査もタスクにできる。観察事実だけを残さず、判断したい問いと、判断をもって完了とすることを明示する。
- 粗いメモで起票してよいが、着手前に目的と受け入れ条件を読める粒度へ詰める。
- バージョン番号を付ける場合は SemVer に従う。
- タスクは `###` 見出しでグループに分ける。バージョンが確定したグループだけ、`### v0.11.0 名前` のように見出しへ番号を付ける。
- グループが大きい場合は、`####` 見出しでさらに分割してよい。

## タスク

### v0.19.2 テストの見直し

- [x] `principle-test-verification-policy` に沿って既存テストの保持価値を見直し、重複や実装詳細への依存で価値が低いケースを簡略化・統合・削除する。完了: 重要な振る舞い・既報の回帰・プロジェクト固有の検証条件を保ち、変更したテストの判断理由と残る検証方法を説明できる。
- [x] CLI テストの stdout/stderr capture で pipe を書込みと並行して読み、出力量による停止を防ぐ。`cmd/mdhop/cli_test.go`。完了: pipe 容量を超える出力を capture してもテストが完了し、標準出力・標準エラーが元に戻る。出典: maintenance-audit A5-3（verify-finding 確認済み）。

### v0.19.3 ファイル操作と索引の整合性

#### ファイル・設定の安全性

- [x] 同じ vault で build が重なっても一時 DB を相互に削除・公開しない。`internal/core/build.go`。完了: 並行 build の失敗・成功のいずれでも、公開された index は完成済みの一貫した DB である。出典: A3。
- [ ] `add --file` が `../` を通じて vault 外の note を読み込み・登録しないようにする。`internal/core/add.go`。完了: vault 外の相対 path を拒否し、外部内容が nodes/meta に入らない。出典: C8。
- [ ] `set`、add の自動 rewrite、convert 等の書き換え系コマンドが symlink 経由で vault 外の Markdown を変更しないようにする。disk path 解決と `internal/core/rewrite.go` の書き込み境界。完了: vault 内 symlink を対象にしても外部ファイルの内容が変わらない。出典: C9/E9。
- [ ] directory move で未登録 asset の移動先が既存なら上書き前に拒否する。`internal/core/move_dir.go`、`move_load.go`。完了: 衝突時に移動先の元内容と移動元の両方が保たれる。出典: D1。
- [ ] move の destination 祖先 symlink から vault 外へファイルを移動しないようにする。`internal/core/move_dir.go` と path 検証。完了: vault 外を指す destination symlink を通る移動は外部ファイルを作らずに拒否される。出典: D7。
- [ ] `init-meta --write` の一時設定ファイルを実行ごとに安全に所有し、既存 `.tmp` や symlink の上書きと並行実行の衝突を防ぐ。`cmd/mdhop/init_meta.go`。完了: 既存の `mdhop.yaml.tmp` とその参照先を変更せず、重複実行後も公開設定が一つの完成した結果になる。出典: E10/F3。

#### メタデータ・索引の整合性

- [ ] NFC の index path に対応する NFD 実ファイルを head・snippet・anchor 診断・directory meta-check でも読めるようにする。`internal/core/query_content.go`、`diagnose.go`、`meta_check.go`。完了: 正規化形式を区別する filesystem 上で build 済みの実ファイルを参照でき、誤った missing/broken 診断を出さない。出典: B4。
- [ ] flow mapping の `set` で指定外の同一行キーを消さない。`internal/core/set.go`。完了: `{status: draft, title: A}` の status 更新後も title が保持される。出典: C1。
- [ ] NFD 名の実ファイルに対する `delete` の存在確認と `--rm` を実 disk path で行う。`internal/core/delete.go`。完了: 正規化形式を区別する filesystem でも、削除成功時に対象ファイルと index の状態が一致する。出典: C3。
- [ ] frontmatter wikilink を add・move・disambiguate で書き換えたとき、edge・mtime とともに meta 値も現在のファイル内容へ更新する。`internal/core/add.go`、`move_dir.go`、`disambiguate.go` と共有更新処理。完了: 各コマンド直後の meta 出力・検索・検査が disk と一致し、full build で結果が変わらない。出典: C5/D3/E8。
- [ ] `set` で引用が必要な既存 YAML key を編集しても有効な frontmatter を保つ。`internal/core/set.go`。完了: `"a: b": old` の更新後も同じ key と他の metadata が解析・索引される。出典: C6。
- [ ] `set` の単一行 scalar 判定で後続キーまでの空行・独立コメントを値の行数に含めない。`internal/core/set.go`。完了: scalar と次キーの間に空行やコメントがあっても対象値だけを更新できる。出典: C7。
- [ ] DB 利用の disambiguate で phantom link を実在 path に書き換えたとき、edge の target_id も更新する。`internal/core/disambiguate.go`。完了: 実行直後の outgoing/backlinks/resolve が書き換え後の実在 note を指し、full build と一致する。出典: E1。

### v0.19.4 リンク解析と書き換え

#### 解析・リンク解決

- [ ] 複数 backtick の inline code 内にあるリンク・タグを索引せず、rewrite 系コマンドでもその内容を変更しない。`internal/core/parse.go`、`rewrite.go`。完了: 二重 backtick のコード例が build の edge/tag に入らず、simplify 後も内容が同じである。出典: 全コードレビュー A1/E7。
- [ ] Unicode 大文字を含む path link の build と resolve で解決規則を揃える。`internal/core/resolve.go`、`resolve_maps.go`。完了: `sub/École.md` へのリンクが build 後の resolve でも同じ実在ノードに解決する。出典: A2。
- [ ] add と move の phantom 昇格時に、basename が同じだけの別 path link を実在ノードへ付け替えない。`internal/core/move_apply.go` と各 caller。完了: `[[missing/X]]` は `other/X.md` の add・move 後も phantom のままで、full build と同じ graph になる。出典: C4/D4。

#### move・rewrite

- [ ] 前後空白を含む相対 Markdown link の参照先を move 後も保つ。`internal/core/move_link.go`。完了: `[link]( ./B.md )` を持つ note を移動しても B.md への参照が維持される。出典: D2。
- [ ] 大小文字の異なる相対 path link が同じ移動セットの note を指す場合、移動後の実 path へ書き換える。`internal/core/move_link.go`。完了: `[[./b]]` が `B.md` を指す directory move 後も、full build で同じ B.md に解決する。出典: D5。
- [ ] 大小文字だけが異なる basename 衝突でも、move の collateral rewrite が既存参照を取りこぼさないようにする。`internal/core/move_rewrite.go`。完了: `readme.md` と新しい `README.md` の衝突後も旧リンクの意味が保持され、full build が曖昧リンクで失敗しない。出典: D6。
- [ ] `disambiguate --scan` で同 stem の asset link を note link に書き換えない。`internal/core/disambiguate.go`。完了: `image.md` があっても `[[image.png]]` と実在 asset path link が保持される。出典: E2。
- [ ] `disambiguate --scan` で dotted note basename を正しく照合する。`internal/core/disambiguate.go`。完了: `Note.v1.md` を対象に `[[Note.v1]]` の必要な書き換えが実行される。出典: E3。
- [ ] convert で path が明示された asset を別ディレクトリの同名 note から誤分類しない。`internal/core/convert.go`。完了: `assets/photo.png` と `notes/photo.png.md` の共存時も asset link の変換先は asset のままである。出典: E4。
- [ ] 括弧を含む note link の convert 出力を、後続 build でも同じ参照先として解析できるようにする。`internal/core/convert.go`、`parse.go`。完了: `[[Meeting (weekly)]]` の convert 後の build が既存 note への edge を作る。出典: E5。
- [ ] template move の実行後に、事前 plan ではなく実際の移動結果を text/JSON に返す。`cmd/mdhop/move.go`。完了: plan と実行の間に metadata が変わっても、成功時に報告する destination が実際の移動先と一致する。出典: F4。

### v0.19.5 検索・CLI・仕様

#### 検索・診断

- [ ] `--where` の右辺に `=` や `!=` が含まれても、左から最初の演算子を条件として解釈する。`internal/core/where_parse.go`。完了: URL を含む LIKE と演算子文字を含む文字列等値検索が意図したキー・値で動く。出典: B1。
- [ ] `--where` の LIKE 右辺にある末尾空白を保持する。`internal/core/where_parse.go`。完了: `title~% ` が末尾空白を持つ値だけに一致する。出典: B2。
- [ ] `meta-validate` が同一キーの異なる不正 list 値を各 value・line 付きで返す。`internal/core/meta_validate.go`。完了: 2 個の不正値を含む list で両方の違反が報告される。出典: B3。
- [ ] 同一秒内に短縮された source の古い行番号で snippet を取得しても panic しない。`internal/core/query_content.go`。完了: 保存行番号が現在の行数を超える場合に制御された結果または error を返し、CLI が runtime panic で終了しない。出典: B5。
- [ ] 8 桁を超える小数部を持つ number 値の sort・比較・等値を数値として整合させる。`internal/core/meta.go` と検索条件。完了: `-1.000000001 < -1` と `1.000000000 = 1` が正しく扱われる。出典: C2。

#### 設定・CLI・仕様

- [ ] `meta.types: null` を持つ既存設定へ `init-meta --preset --write` した際、報告した型定義を YAML に保存する。`internal/core/init_meta_yaml.go`。完了: added 件数と再読込した `meta.types` の内容が一致する。出典: E6。
- [ ] コマンドの余剰位置引数を拒否し、後続の `--dry-run` や範囲指定が黙って無効にならないようにする。`cmd/mdhop` の flag 解析。完了: 位置引数を混ぜた convert/repair/simplify 等がファイルを書き換えずに引数 error を返す。出典: F1。
- [ ] `init-meta` の stdout YAML 出力失敗を終了コードへ反映する。`cmd/mdhop/init_meta.go`。完了: stdout への書き込みが失敗した場合、成功扱いにならない。出典: F2。
- [ ] `simplify` の仕様を現行の frontmatter 書換え範囲に合わせる。`docs/specs/overview.md`。完了: quoted frontmatter wikilink は対象、raw `frontmatter_path` と tag は対象外と読め、実装・テスト・ADR 0022 と矛盾しない。出典: maintenance-audit A6-1（verify-finding 確認済み）。

### 新機能：関連検索のハブ除外

- [ ] 多くの note とつながる経由 note を除外して関連検索のノイズを抑える機能について、利用場面、除外対象・接続数の数え方、設定/既定値、既存 `max_via_per_target` との関係、受入例を検討する。`docs/rules/02-requirements.md`、`internal/core/query.go`、`query_fetch.go`、`cmd/mdhop/query.go` を対象に、`via_max_degree` の flag 名や具体アルゴリズムは先に固定せず、仕様確定前に実装へ進まない。出典: GROK:D6。完了: 仕様と実装 scope を確定する。

### 低優先度：対応要否を再評価する候補

- [ ] basename の resolve と `query --name` の全 note 走査が、実用規模や反復 lookup で問題になるか測り、旧 NFD index との互換を含めて改善の採否を決める。`internal/core/resolve.go`、`query_entry.go`。完了: 負担と改善費用を比較し、対応要否を判断する。出典: A4。
- [ ] 少数行の head 取得でノート全文を読み込む負担と、読み取り量を減らす場合の出力互換条件を確認する。`internal/core/query_content.go`。完了: 大きな note の利用実態と未閉鎖 frontmatter の扱いを踏まえ、局所改善の採否を決める。出典: B6。
- [ ] move 失敗時のファイル復元で元の mtime も戻し、復元自体による stale 判定を防ぐ。`internal/core/rewrite.go`、`move_dir.go`。完了: 内容・permission・mtime の復元が成功した場合、原因を除いた再試行が rollback 自身を理由に stale 扱いされない。出典: D8。
- [ ] E5 の括弧付きリンク修正時に、通常解析と convert の Markdown リンク字句走査を共有する必要があるか判断する。`internal/core/parse.go`、`convert.go`。完了: 必要な同一構文認識だけを共通化するか、現行の別実装を維持するか決め、自己リンクの扱いの違いを保つ。出典: maintenance-audit A2-1（verify-finding 確認済み）。
- [ ] D6 の basename 衝突修正時に、unique/root 優先規則の重複を解決器へ集約する必要があるか判断する。`internal/core/resolve_maps.go`、`move_rewrite.go`、`link_ambiguity.go` など。完了: build・add・move・曖昧性判定の意味の違いを確認し、共通化する規則の範囲を決める。出典: maintenance-audit A3-1（verify-finding 確認済み）。
