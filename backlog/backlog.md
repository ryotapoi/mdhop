# Backlog

## 運用ルール

- Backlog には、今後着手でき、完了を判定できる作業を `- [ ]` 形式のタスクとして置く。
- 対応要否を決める調査もタスクにできる。観察事実だけを残さず、判断したい問いと、判断をもって完了とすることを明示する。
- 粗いメモで起票してよいが、着手前に目的と受け入れ条件を読める粒度へ詰める。
- バージョン番号を付ける場合は SemVer に従う。
- タスクは `###` 見出しでグループに分ける。バージョンが確定したグループだけ、`### v0.11.0 名前` のように見出しへ番号を付ける。
- グループが大きい場合は、`####` 見出しでさらに分割してよい。

## タスク

### v0.21.0 関連探索と単体情報取得

- [x] [v0.21.0 の実装仕様](v0.21.0.md)を確定する

  完了: 関係・CLI・設定・出力・抜粋・移行・異常値と実装順を同ファイルへ記録した。

以下の段階を上から同一バージョンで実施する。中間段階では新機能が未完成でもよく、最終段階までリリースしない。ただし各段階の完了時には、その段階で触れた契約の集中テストと、`docs/rules/verification.md` に従う `go test ./...`・`go build ./...` を通す。CLI を変更した段階は実バイナリで正常系・異常系、stdout / stderr、終了コードを確認する。型や API の変更で既存呼出側が壊れる場合は同段階で直し、後続段階に壊れた build や恒久的な互換 shim を渡さない。新仕様の全受入確認は最終段階で行う。

#### 1. 設定と絞り込みの土台

- [x] [設定と照合規則](v0.21.0.md#設定と照合規則)の目的別 config、旧 exclude fallback、GLOB と型付き経由先の解析を実装する

  完了: config の欠落と明示空を区別し、hide / via include / via exclude を独立合成できる。旧 exclude は新 via exclude キー不在時だけ fallback となり、search の既存契約は変わらない。異常値と競合、GLOB の `/`・`?`・`[]`、型付き経由先の正規化と完全一致の集中テスト、および `go test ./...`・`go build ./...`・`go vet ./...` が通った。実バイナリで search の旧 exclude・`--no-exclude`・明示 CLI 条件と設定構造エラーの stdout / stderr・終了コードを確認した。新 query CLI と関係生成への接続は段階 2 で行う。

#### 2. query の関係・選択・ページ・出力

- [x] [CLI 契約](v0.21.0.md#mdhop-query)と[関係を作って返すまで](v0.21.0.md#関係を作って返すまで)に従い、query core、CLI、JSON / text formatter を一緒に変更する

  完了: 三関係の全件取得、選択欄の省略と空配列、全入口で同じ twohop 向き、タグ outgoing、hide と via の独立、typed node と全経由先、単一関係ページ・next_offset が動く。旧 query オプションは削除され、既存呼出側と既存テストを同段階で更新する。head / snippet の新プレビュー機能は段階 4 で追加し、この段階の完了条件には含めない。選択0件・非表示経由のみ・重複対象・末尾ページ・不正入力、および旧上限を超える125対象・16経由先の集中テストが通った。`go test ./...`・`go build ./...`・`go vet ./...` と実バイナリの正常系・異常系27件を確認し、JSON単独解析、stdout / stderr・終了コード、query による file / DB の無変更、search の旧 exclude / no-exclude と head の不変を確認した。旧入口previewのテストは共有本文helperへ移し、NFD・stale・missing の保証を保持した。

#### 3. inspect による単体情報取得

- [x] [inspect の契約](v0.21.0.md#mdhop-inspect)を CLI・core・JSON / text に実装する

  完了: 索引上の note 一件の tags / meta と任意 head を返し、選択済みの空欄（tags / head は `[]`、meta は `{}`）と未選択欄の省略を JSON / text で区別する。属性選択と head 指定の独立、親タグと葉タグの表示差、複数 meta 値と引用、hide / via / 旧 exclude の非適用、不正 fields / head、未登録 file・非 note・索引なしを集中テストと実バイナリで確認した。本文変更・削除後も head なしでは索引属性を返し、head ありでは stale / missing エラーになる。正整数の最大 head 行数でも overflow しない。`go test ./...`・`go build ./...`・`go vet ./...` と実バイナリ33件が通り、JSON 単独解析、stdout / stderr・終了コード、inspect による file / DB の無変更、索引の自動作成なしを確認した。query preview と正本文書の移行は段階4・5に残す。

#### 4. query の head / snippet

- [x] [head / snippet の範囲](v0.21.0.md#head--snippet-の範囲)に従い、返す関係に必要な本文プレビューを追加する

  完了: head は返却 note のみ、snippet は出力関係に属する実リンク出現のみを付ける。backlinks は対象→入口、outgoing は入口→対象、twohop は対象→可視 via の生行を返し、frontmatter タグ・リンク、型付き入口・via、同一行の別 edge と重複する文脈、複数 seed でも増殖しない出現を確認した。`--path` / `--where` は対象だけ、`--link-key` は直接 edge だけへ適用し、hidden・ページ外・先読み・未選択関係の不要本文を読まない。必要な本文の missing / stale と最大 int の snippet 文脈も確認した。JSON の指定済み空と省略、text の所属・引用 escape、preview flag の 0 / 不正値を集中テストで固定した。`go test ./...`・`go build ./...`・`go vet ./...` と実バイナリ27件が通り、stdout 単独 JSON 解析、stdout / stderr・終了コード、file / DB の無変更、search の共通 head / 旧 exclude の不変を確認した。正本文書の移行と版全体の最終受入は段階5に残す。

#### 5. 正本・移行・最終受入

- [x] [実装仕様](v0.21.0.md)と実装結果を照合し、`docs/specs/`・利用者向け移行案内を更新して v0.21.0 を検証する

  完了（評価対象）: query / inspect の正本と rules の旧記述、README 両版の移行案内を凍結仕様・実装へ合わせた。全受入8項目を既存集中テストに対応付け、旧 query CLI 拒否、config fallback と明示空、search 不変、非 note 入口、タグ経由、JSON / text の型・escape、対象ページ、preview の所属・本文読取境界を実バイナリ79件で確認した。正本の JSON / text 例も別 vault の実行結果へ照合した。ローカル macOS で集中テスト、`go test ./...`・`go build ./...`・`go vet ./...`・実バイナリ build が成功し、stdout / stderr・終了コード・file / DB 無変更を確認した。対象commitのremote CIは未起動でUbuntuは未確認、前段からの検証限界として保持する。index同時更新保証は対象外。便利な一括scriptは製品化せず、release / tag 公開は行っていない。

### リンク解釈の不具合修正

- [x] 表内の自己リンクを wikilink に変換するとき、表構造と fragment の意味を保持する

  分類: 明確な不具合で修正必須。通常の `convert --to wikilink` が実ファイルを書き換え、表構造とリンク先 fragment を変えてしまう。`15ea54c..dea83cc` の変更で導入され、`dea83cc` の CLI と再 build で再現を確認済み。

  再現: `Source.md` に次の一列表を置き、build 後に同ファイルを wikilink へ convert し、再 build する。

  ```markdown
  | [shown](#H%5C) |
  | --- |
  ```

  現象: decoded fragment 末尾の backslash 1 個と表内 alias separator の backslash が連続し、pipe 直前が backslash 2 個になる。header が2列、delimiter row が1列として解釈され、自己リンク edge の `in_table=0`、`subpath` 末尾が backslash 2 個となる。resolve の JSON でも元の fragment と異なる値を返す。

  修正範囲: `internal/core/convert.go` の `convertMarkdownToWikilink` にある自己リンクの alias 分岐。通常リンク側で使う `internal/core/markdown_destination.go` の `tableWikiAliasSafe` と同じ安全条件を適用できる。新しい表解析や変換方式を追加せず、同じ意味を表現できない場合は原文を保持する既存契約（`docs/specs/overview.md`「再出力と既存 index」）に従う。

  受入条件: 上記入力は convert 後も原文を保持し、再解析しても一列表であることと decoded fragment 末尾の backslash が1個であることを確認する。再 build で fragment-only Markdown destination は既存仕様どおり graph に含めず、破損した自己リンク edge を生成しない。表内の安全な自己リンクの alias 変換、alias 不要の自己リンク、通常リンクと表外リンクの既存変換を保つ。

  完了: 自己リンクの alias 分岐にも既存の `tableWikiAliasSafe` を適用し、表内で末尾 backslash が奇数個の場合は原文を保持する。安全な alias 変換（末尾 backslash なし・偶数個）、alias 不要、表外を回帰テストで固定し、対象の一列表の表文脈と復号済み fragment を確認した。集中テスト、`go test ./...`、`go build ./...` と実バイナリの build → convert dry-run → convert → 再 build が成功し、原文 bytes、dry-run の file / DB 無変更、保持した fragment-only Markdown link の edge 不生成と resolve の `link not found`、安全な表内自己リンクの変換・resolve を確認した。

- [x] destination 内の escape された backtick によるリンク索引の欠落を直す

  分類: 不具合で、既存の本文 scanner の escape 判定を局所的に補えば保守負担を増やさず修正できる。変更前から存在し、`dea83cc` の build / update / resolve で再現を確認済み。新しい parser、CLI、DB 項目は不要。

  再現: 実在するノート ``A`B.md`` と `C.md` を作り、`Source.md` の本文を次の1行にして build または update する（destination の backslash は1個）。

  ```markdown
  [shown](A\`B.md) [later](C.md)
  ```

  現象: `internal/core/parse.go` の `stripInlineCode` が escape された backtick を code span の開始と扱い、閉じ backtick がないため行末まで空白化する。build / update は成功するが両リンクが索引から欠落し、outgoing は空、resolve は `link not found` となる。backtick を `%60` とした対照入力では両ノートへのリンクが索引に入る。destination の ASCII punctuation backslash escape を復号する仕様（`docs/specs/overview.md`「リンク解釈（互換性）」）に反する。

  受入条件: 上記入力を build / update すると ``A`B.md`` と `C.md` への両 edge が作られ、outgoing と原文指定の resolve で確認できる。`stripInlineCode` の開始 delimiter 判定で backslash の奇偶による escape を扱い、実際の code span 内のリンク除外と位置保持を維持する。escape されない backtick と連続 backslash の対照ケースも確認する。

  完了: `stripInlineCode` の開始 backtick に直前の連続 backslash の奇偶判定を加え、奇数個なら原文を保持する。1・3個と0・2個の対照、実 code span の除外と closing delimiter 直前の backslash、masking の byte 位置、復号済み target・raw link・行位置を回帰テストで確認した。修正前の回帰失敗と修正後の集中テスト、`go test ./...`、`go build ./...` の成功を記録し、実バイナリで再現入力の build と `%60` 対照からの update における両 edge・outgoing・原文 resolve、source file 無変更、未索引 fragment の resolve 失敗と DB 無変更を確認した。

### 今後の検討

- [ ] 全量 build と変更検出・差分反映を比較し、自動差分更新を追加する価値を判断する

  目的: Markhop アプリから気軽に索引を更新する際の待ち時間を減らす価値があるかを判断する。エージェントによる利用では全量 build で十分でも、アプリの繰り返し操作では応答時間の差が重要になり得る。利用者が更新対象を列挙せずに索引を最新化する操作について、現行の全量 build で十分か、変更検出を含む差分反映に実用上の利点があるかを判断する。差分機能の実装を前提にせず、採用しない判断も完了とする。

  調査: 代表的な Vault と規模・本文量・変更割合を変えたケースで、現行 build の列挙・ファイル読み取り・解析・DB 作成にかかる時間を測る。更新時刻等の確認コストも含めて、全量 build、既存の status と update / add 等の組み合わせ、自動差分反映の最小案を比較し、削減できる時間・必要になる保守負担・利用頻度を整理する。実測上十分速い場合は現行方式を維持する。

  検討事項: 「前回 build 時刻より mtime が新しいファイルだけ」では、削除、mtime を保持した追加・移動・復元、同一秒の変更、build 中の変更を取りこぼし得る。既存のファイル別 mtime を用いる方法との違いと、保証できる範囲を確認する。また、変更していないノートでも、参照先の追加・削除や basename 衝突で解決結果が変わること、設定・解析規則・索引形式の変更では全量再生成が必要になることを踏まえ、全量 build と同じ関係を保つために必要な処理を評価する。

  完了: 測定条件と結果、採用 / 見送りの判断と根拠をまとめる。採用する場合は、解決する利用上の問題、最小の操作・変更検出規則、全量 build へ戻す条件と受入条件を明確にし、実装を別タスクとして起票する。
