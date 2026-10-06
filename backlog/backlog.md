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

#### 6. 検証済み不具合の局所修正

検証元: `15ea54cb6ef13d1b8324c0eb97d5c243af32b81e..eea5097ed9af6d0023f3115e4d167a29fb7908e5` を Sol の fresh-review で確認し、別の Sol fresh subagent で候補5件の事実・到達経路・対応価値を検証した。全5件を対応候補と判定した。FR-001（`invariants-1`）と FR-003（`requirements-002`）は今回の解析変更で問題が起きる入力が増え、FR-004（`maintenance-1`）と FR-007（`local-1`）は今回の差分で導入された。FR-008（`contracts-2`）は差分前から同じ経路に存在する。静的追跡による確認であり、実操作による再現と性能測定は未実施。

- [x] escaped backtick 後のリンクを解析と同じ範囲で書き換える（FR-001）

  現象・根拠: 本文が ``\`[[A]]`` の行を build して `A.md` を `B.md` へ move すると、解析済みの `[[A]]` を rewrite が inline code として扱い、本文を置換しない。move は成功し、外部 edge の raw_link だけが `[[B]]` へ更新される。同じ位置の Markdown link の convert でも、未置換を Rewritten として報告する。`internal/core/parse.go` の `stripInlineCode` は開始 backtick 前の連続 backslash の奇偶を判定するが、`internal/core/rewrite.go` の `replaceOutsideInlineCode` は判定しない。調査時点では静的追跡で確認済み、実操作は未確認だった。

  修正範囲: `replaceOutsideInlineCode` の開始 delimiter 判定を既存の解析側と揃える局所修正。既存の `inlineCodeEnd` を維持し、新しい parser・書き換え方式・DB 項目は追加しない。

  受入条件: 奇数個（1・3個）の backslash 後の backtick に続くリンクを move / convert が実際に置換し、move 後の本文・DB edge・resolve と convert の更新報告が一致する。偶数個（0・2個）の backslash 後の実 code span 内ではリンクを置換せず、同じ長さの delimiter で閉じる既存の判定を保つ。dry-run の本文・DB 無変更も確認する。

  完了: `replaceOutsideInlineCode` の開始 backtick に解析側と同じ連続 backslash の奇偶判定を加えた。1・3個の回帰テストの修正前失敗と修正後成功、0・2個の code span 保護、既存 delimiter-run 判定、集中テスト、`go test ./...`、`go build ./...` を確認した。実バイナリで修正前の不一致を再現し、修正後の move 直後の本文・DB edge・resolve と convert の本文・4件の更新報告が一致すること、convert が DB を変更せず再 build 後に索引と整合すること、両 dry-run の file / DB 無変更と code-only link の resolve 失敗を確認した。

- [x] wikilink の alias 内を Markdown 自己リンクとして変換しない（FR-003）

  現象・根拠: `[[Target|[shown](&num;H)]]` に `convert --to wikilink` を実行すると、自己リンクの追加 scanner が alias 内を独立リンクとして収集し、本文を `[[Target|[[#H|shown]]]]` へ変更する。`internal/core/convert.go` の `parseLinksForConvert` は `clean` をそのまま追加 scanner に渡し、scanner は opening `[[` のみを読み飛ばす。通常解析は `internal/core/parse.go` の `maskWikiLinks` で wikilink 全体を除外しており、境界が不一致。実バイナリで alias と外側が同一 raw のときの誤変換を再現済み。通常 convert と move incoming / outgoing の共有本文置換でも alias や Markdown 表示文への置換漏れを確認した。

  修正範囲: 追加 scanner の入力に既存の `maskWikiLinks` を再利用する。共有本文置換も Markdown に限って wikilink と表示文を除外し、既存の bracket / destination / inline code helper で独立した occurrence を照合する。新 parser や汎用位置追跡モデルは追加しない。

  受入条件: 上記入力と literal `#H` を使う対照入力の既存 wikilink・alias を原文のまま保持し、alias 内を Rewritten として報告しない。同じ行の wikilink 外にある通常の Markdown 自己リンクは変換する。byte 位置・行番号、表文脈、frontmatter / fence / inline code の除外、dry-run の本文・DB 無変更を維持する。

  完了: entity / literal の自己リンク、alias だけの入力、同一行の外側リンクと Markdown 表示文を確認した。恒久回帰の修正前失敗と修正後成功、表・multibyte・frontmatter / fence / inline code の保持、move incoming / outgoing の本文・DB raw・resolve 整合を確認した。`go test ./internal/core/`、`go test ./...`、`go build ./...`、`go vet ./...` が成功。表示文中の inline code / wikilink を mask 済み raw で上書きしない原文照合も恒久回帰と実 CLI convert / move で確認した。実バイナリで両変換方向・embed・escaped backtick の対照、convert と template move dry-run の file / DB 無変更、actual convert の DB 無変更と再 build 後の整合、move 直後の索引整合、および不正入力の失敗と無変更を確認した。

- [x] 同一 query の snippet 生成で同じ本文の全量読取・保持を重複させない（FR-004）

  現象・根拠: `query --relations outgoing --include-snippet 0` の複数対象や、同一対象の複数 via で、関係ごとに同じ本文を全文読み直す。`internal/core/query_preview.go` は関係単位で `readSnippets` を呼び、`internal/core/query_content.go` の既存 fileCache は呼出しごとに作られる。返す `Lines` も全文配列の subslice のため、本文量と関係数に比例して読取・保持が重複する。構造は静的追跡で確認済み、実時間・メモリ影響は未測定。

  修正範囲: 既存の path ごとの本文 cache を `addQueryPreviews` の単一 query の snippet 生成中だけ共有し、抜粋の `Lines` は必要範囲をコピーして返す。製品側の `readSnippets` 呼出しは同関数内の一箇所のみ。局所的な受渡しと既存読取 helper の変更に限定し、新しい cache 抽象・長期 state・head との統合は追加しない。

  受入条件: 複数 outgoing と同一対象の複数可視 via について、同じ本文の全量読取は query 内で一回となり、各 snippet の生行・行番号・所属・重複出現規則は変わらない。返却 `Lines` は全文配列を保持しない。必要な本文の初回読取で missing / stale を検出し、NFD path、行範囲検証、hidden via・ページ外・先読み・未選択関係の不要本文を読まない保証を維持する。snippet 未指定時はこの共有読取を行わない。

  完了: query 内だけの本文 cache を全 snippet 関係で共有し、返却 `Lines` を独立コピーにした。初回読取後の削除・変更による再読取回避、独立 cache の missing / stale、cache hit の行範囲検証、抜粋間と本文 cache の非 alias を恒久回帰で確認した。既存の関係所属・重複出現・不要本文読取回避・NFD の集中テスト、`go test ./...`、`go build ./...` が成功。時間・メモリ改善量は未測定。

- [x] quoted frontmatter で表現可能な backtick を含む wikilink の必須書き換えを許可する（FR-007）

  現象・根拠: `A.md` と `related: "[[A]]"` を持つ `Source.md` を build 後、`A.md` を ``Z`Q.md`` へ move すると、本文リンクがなくても `cannot preserve wikilink destination` エラーになる。`internal/core/rewrite.go` の `rewriteRawLink` は本文と frontmatter に共通の `wikilinkRepresentable` を適用し、backtick を一律拒否する。一方、`internal/core/parse_frontmatter.go` の quoted scalar 解析は本文の code span scanner を通らず、`internal/core/rewrite_frontmatter.go` の source 対応検証でも通常の backtick は表現可能。操作の拒否は今回追加された guard による。

  修正範囲: 本文と quoted frontmatter の表現条件を既存の link type と解析・source 対応検証に沿って区別する局所修正。本文で表現不能な宛先の拒否と、frontmatter の YAML 意味保存・操作前検証は維持する。複雑な YAML scalar 全般への対応拡張は含めない。

  受入条件: 単一行 quoted scalar の上記 move が成功し、書き換え後の本文・DB edge・原文 resolve・再 build 後の解決先が ``Z`Q.md`` で一致する。backtick を含む既存 target / subpath の必須書き換えも意味を保持する。本文の表現不能ケースと YAML source 対応を証明できないケースは file / DB 更新前に拒否し、失敗時と dry-run の無変更を確認する。

  完了: 共通 rewrite と移動 note 自身の相対 outgoing rewrite の表現判定を LinkType で区別し、quoted frontmatter の backtick を許可した。single/double quote、既存 target/subpath と alias の保持、書き換え直後と再 build 後の原文・DB edge・exact raw resolve の一致を恒久回帰で確認した。本文の表現不能拒否と YAML 原文対応不能拒否、actual / template dry-run の file・DB・一時 file 無変更を確認した。集中テスト、`go test ./...`、`go build ./...`、`go vet ./...` と実バイナリの正常・異常経路が成功。複雑な YAML scalar の対応範囲は拡張していない。

- [x] 末尾の閉じ角括弧で参照先が変わる wikilink の必須書き換えを拒否する（FR-008）

  現象・根拠: `A.md` と本文 `[[A]]` を持つ `Source.md` を build 後、`A.md` を `B].md` へ move すると、本文を `[[B]]]` に変更して成功する。`internal/core/markdown_destination.go` の `wikilinkRepresentable` は内部の `]]` だけを拒否し、単独の末尾 `]` を許す。`internal/core/rewrite.go` の wrapper 連結後は `internal/core/parse.go` の `wikiLinkSpans` が最初の `]]` で閉じるため、再解析 target は `B` となる。DB は移動先を指したままで、再 build により参照先が変わる。開始 SHA でも同じ構文を生成する既存不具合であり、今回の差分による導入ではない。

  修正範囲: 実際に出力する wikilink の wrapper・alias・subpath と既存 parser の境界に沿って表現可否を判定し、同じ意味を表現できない必須 rewrite は更新前に拒否する。新しいリンク構文や parser は追加しない。判断の正本は `docs/specs/overview.md`「再出力と既存 index」。

  受入条件: alias / subpath のない上記 move は file / DB を変更せずエラーになる。target または subpath の末尾 `]` が closing wrapper と結合する境界を確認し、表現可能な対照ケースは拒否せず、生成後の再解析で target / subpath の意味を保持する。dry-run と失敗時の無変更を確認する。

  完了: closing wrapper に接する target/subpath の末尾 `]` を共有 rewrite と相対 outgoing rewrite で拒否し、subpath・alias・表内 alias が境界を隔てる表現可能な出力を保持した。convert は拡張子除去と alias 決定後に判定し、不能時は原文を保持する。修正前に既報出力を検出する回帰を確認し、本文・quoted frontmatter の actual / template dry-run 失敗時の file・DB・残留 file 無変更、有効 move の本文・DB raw・resolve と再 build 後の target/subpath 一致を恒久回帰と実バイナリで確認した。FR-007 の backtick 回帰、集中 test、`go test ./...`、`go build ./...`、`go vet ./...` が成功。新 parser・新構文は追加していない。

- [x] autolink で始まる GFM 表の文脈を保持する（FR-002）

  修正範囲・受入条件: `tableBlockBoundary` の `<` 一律判定を表境界に必要な HTML block start の局所分類へ変える。URL / email autolink と inline HTML を先頭 cell に持つ有効表では escaped pipe alias と原文位置を保ち、実 HTML block start は表を終了する。共有本文 scanner、frontmatter / fence / code span の扱いは変えない。

  完了: autolink / email / inline HTML の header・body、実 HTML block と block に似た inline cell の対照を恒久回帰テストで固定した。集中テストと全 gate が成功し、実バイナリの build → resolve → convert dry-run / actual → 再 build で、表内 alias の解釈と変換後の解決先、実 HTML 境界対照、dry-run の file / DB 無変更を確認した。

- [x] twohop の経由先選択を backlinks 展開前に適用する（FR-005）

  修正範囲・受入条件: 入口 outgoing の経由先を既存 `AllowsVia` で先に選び、許可した経由先だけ展開する。型付き指定、包含・除外、対象 filter、direct 限定の link-key、hide による表示除外と hidden via による発見、全経由先・安定順を保つ。SQL に predicate を複製しない。

  完了: 既存 query / link-key 回帰と追加 page parity 回帰、全 gate が成功した。実バイナリで型付き via、via 除外、hidden-only relation、JSON / text、stdout / stderr・終了コードと query の file / DB 無変更を確認した。合成 sparse fixture の選択経由先では変更前 3.78ms から 0.212ms、dense では 265ms から 5.31ms に短縮し、公開結果の hash は一致した。

- [x] query のページ対象だけを保持・関係展開する（FR-006）

  修正範囲・受入条件: direct は既存 comparator とフィルタを使い offset + limit + 一件先読みだけを Go 側へ保持する。twohop は対象ページを確定した後、その対象の全 relation を作る。公開結果は全件取得の対象 slice と一致し、next_offset、空配列、offset-only・最大 int、hidden_relation と返却対象だけの preview を保つ。schema / CLI 契約は変えない。

  完了: 三関係の対象 slice・全 relation・next_offset、空・末尾・overflow 境界を恒久回帰で固定し、preview の既存回帰と実バイナリで hidden via・ページ外・先読みの欠落本文を読まないことを確認した。集中テスト、`go test ./...`・`go build ./...`・`go vet ./...`・実バイナリ build と代表 CLI 21件がローカル macOS で成功した。既存8条件の公開結果 hash は baseline / FR-005 単独 / 本候補で一致し、dense limit1 は FR-005 単独の 183.7ms / 36.95MB allocated bytes/op から 68.0ms / 0.446MB に改善した。追加の medium / large page window も公開結果は一致し、dense の limit100 / offset100 は FR-005 単独の 186.8ms → 70.3ms、limit1000 / offset1000 は 190.6ms → 106.7ms へ改善した。sparse の後者は FR-005 単独の 3.65ms → 5.59ms（約1.94ms / 53%増、変更前 baseline 比は約0.91ms / 20%増）となった。全件に近いページの二段階 scan と heap の追加コストを明示し、密な関係の改善とのトレードオフを許容して取得段階 paging を採用した。

  検証限界: 2,500対象の合成 sparse / dense vault、warm cache の測定であり実 vault の速度や peak RSS は保証しない。SQL DISTINCT は全候補を走査し、offset-only・overflow-sized window は全対象取得へ fallback する。33経由先を超える広い入口は未測定。remote CI は未起動、Ubuntu は未確認。実行 command・exit・duration・stdout / stderr と比較値は `tmp/workflow/v021-fixes-20261005/changes/table-query-fixes/validation/`、既存測定は `tmp/workflow/query-optimization-20261005/summary.json` に保存した。

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

- [ ] basename lookup の全行走査を、保守負担を大きく増やさずに高速化する

  目的: `resolve` の basename 解決と `query --name` が索引の note / asset 数に比例して対象 type の全行を読み取るコストを減らす。性能改善は行いたいが、メンテナンス性への大きな悪影響がある案は採用しない。

  現状: `internal/core/resolve.go` の `queryBasenameMatches` は全行を Go 側で NFC 正規化・小文字化して比較する。旧 NFD index name との互換を保つための処理であり、単純な SQL の name 条件への置換では同じ解決結果を保証できない。旧10万 note の合成測定では約56.5ms/op・13.44MB/opだったが、現行版・実 Vault の性能保証ではない。

  実施: 現行版で規模・Unicode表現・同名候補の有無を変えて時間と allocation を測定し、全行走査を減らす最小案を比較する。既存の一意 / ルート優先、note / asset 優先、曖昧時の挙動と旧 NFD name 互換を維持する。索引の追加状態・移行処理・キャッシュ同期などに必要な保守負担を評価し、効果に対して複雑さが小さい案を実装する。全体のリンク解決規則の再整理は含めない。

  完了: 改善前後の同条件測定で効果を示し、既存の解決契約と互換性を回帰検証する。保守負担が大きい案しか得られない場合は、比較結果と見送り理由を記録して完了とする。

- [ ] 全量 build と変更検出・差分反映を比較し、自動差分更新を追加する価値を判断する

  目的: Markhop アプリから気軽に索引を更新する際の待ち時間を減らす価値があるかを判断する。エージェントによる利用では全量 build で十分でも、アプリの繰り返し操作では応答時間の差が重要になり得る。利用者が更新対象を列挙せずに索引を最新化する操作について、現行の全量 build で十分か、変更検出を含む差分反映に実用上の利点があるかを判断する。差分機能の実装を前提にせず、採用しない判断も完了とする。

  調査: 代表的な Vault と規模・本文量・変更割合を変えたケースで、現行 build の列挙・ファイル読み取り・解析・DB 作成にかかる時間を測る。更新時刻等の確認コストも含めて、全量 build、既存の status と update / add 等の組み合わせ、自動差分反映の最小案を比較し、削減できる時間・必要になる保守負担・利用頻度を整理する。実測上十分速い場合は現行方式を維持する。

  検討事項: 「前回 build 時刻より mtime が新しいファイルだけ」では、削除、mtime を保持した追加・移動・復元、同一秒の変更、build 中の変更を取りこぼし得る。既存のファイル別 mtime を用いる方法との違いと、保証できる範囲を確認する。また、変更していないノートでも、参照先の追加・削除や basename 衝突で解決結果が変わること、設定・解析規則・索引形式の変更では全量再生成が必要になることを踏まえ、全量 build と同じ関係を保つために必要な処理を評価する。

  完了: 測定条件と結果、採用 / 見送りの判断と根拠をまとめる。採用する場合は、解決する利用上の問題、最小の操作・変更検出規則、全量 build へ戻す条件と受入条件を明確にし、実装を別タスクとして起票する。
