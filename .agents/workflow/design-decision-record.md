# Design Decision Record

## Intent

設計判断は、後から「なぜその方向にしたのか」を確認するために残す。
実装ログや仕様の再掲ではなく、複数の妥当な選択肢から選んだ理由を記録する。
`docs/decisions/` への記録は ADR として書く。形式は固定しない。

## Product Decision Ledger

Product decision ledger は、PM / デザインリード / QA / ユーザーなどのステークホルダーが別方針を選び得る判断を、作業中に忘れないための一時的なメモ。正本ではない。必要なものだけ `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` へ同期する。

長い作業、Goal、subagent 委任、review 指摘対応をまたぐ作業では、必要に応じて `tmp/workflow/<scope>/product-decision-ledger.md` に残す。短い Change では plan / review 結果 / 最終報告の中に同じ項目を構造化して残してよい。

### Alternative Check

その選択で振る舞い仕様——ユーザーへの約束として `docs/specs/` に書くべき内容——が変わるなら、それは product decision であり、採用案だけで進めず、少なくとも「既存挙動に寄せる案」と「採用案以外の妥当案」があるかを確認する。この定義を Product Decision Ledger の対象の正本とする。

- 例: 表示・文言の意味や既定値、ユーザー操作の結果（完了・削除・復元など）の挙動、データの解釈・保存・同期の意味、同じ操作を複数 surface（UI / 外部 API / 自動化連携）でどう扱うか、用語・概念の意味。
- 対象外: 振る舞い仕様が変わらない「どう実現するか」の選択（モジュール配置、型選択、リファクタ、テスト構造など）。

報告対象にするのは、現在のユーザー依頼、`backlog/backlog.md`、`docs/rules/` / `docs/specs/` / `docs/decisions/` に明記されておらず、`boundary-control` / `design-decision` / `module-boundary` / `thermo-nuclear-code-quality-review` などの判断系 skill でも実装判断として明確に処理できず、Codex がステークホルダー判断に近い選択をしたものに限る。

報告対象にしないもの:

- backlog や docs に明記済みの内容を、その通り実装しただけのもの。
- 判断系 skill の基準で自動判断できる実装寄りの設計判断。例: module / folder / helper / private API / test structure / 局所 refactor。
- 単なる未実装 TODO や、今回の scope 外の adjacent work。

可逆で影響が小さい選択は採用案で進め、Product Decision Ledger の対象なら ledger に残す。複数の妥当案が残り、かつ選択が非可逆（データ保持・削除・マイグレーション・外部公開契約）またはやり直しコストが大きい場合、または正本と矛盾する場合は、ledger に仮案を書いて押し切らず、呼び出し元 workflow の Stop Conditions に従う。

ledger には次を残す:

- **判断**: 何を決める話か。プロダクトの何がどう変わるかの説明。
- **Source check**: backlog / docs / decisions / 現在の依頼に明記があるか。
- **Skill check**: 判断系 skill で実装判断として解けるか。
- **採用案**: Codex が選んで進めた案。
- **別案**: 既存挙動に寄せる案と、他の妥当案。
- **理由と影響**: 採用理由、QA / UX / データ意味への影響。
- **ユーザー判断**: Goal 完了報告へ引き継ぐか。引き継がないなら理由。
- **同期先**: 必要なら docs / backlog / decisions のどこへ反映するか。

各項目は、実装詳細・技術詳細ではなく、マネージャーに伝えるつもりのレベル感（方向性）で書く。

## What Not To Record

- backlog や specs に書かれた要求を、そのまま実装しただけの内容。
- 「既存がそうだから」「差分が小さいから」だけの説明。
- DRY / YAGNI / Clean Architecture などの原則名だけを理由にした説明。
- コードレベルの細部。通常の実装規約や design-decision skill で決められるものは書かない。
- 単なる TODO や後続タスク。必要なら `backlog/backlog.md` に入れる。
- 「後続に回す」というだけの説明。後続にするなら、なぜ今やらない方がよいか、または単に未実装タスクなのかを分ける。

## Granularity

- プロダクトの振る舞い、データ正本、外部 surface、同期、UX、モジュール境界のように、後から変更コストが高いものは記録する。
- 変数名、関数分割、局所 helper の有無など、レビュー時にコードから判断できるものは記録しない。
- 仕様として既に確定した内容は設計判断ではなく、仕様同期または backlog 更新として扱う。
