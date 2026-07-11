# Design Decision Record

## Intent

設計判断は、後から「なぜその方向にしたのか」を確認するために残す。
実装ログや仕様の再掲ではなく、複数の妥当な選択肢から選んだ理由を記録する。

## Product Decision Ledger

Product decision ledger は、PM / デザインリード / QA / ユーザーなどのステークホルダーが別方針を選び得る判断を、作業中に忘れないための一時的なメモ。正本ではない。必要なものだけ `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` へ同期する。

長い作業、Goal、subagent 委任、review 指摘対応をまたぐ作業では、必要に応じて `tmp/product-decision-ledger/<scope>.md` に残す。短い Change では plan / review 結果 / 最終報告の中に同じ項目を構造化して残してよい。

### Alternative Check

次のカテゴリに触れる変更では、採用案だけで進めず、少なくとも「既存挙動に寄せる案」と「採用案以外の妥当案」があるかを確認する。このカテゴリ一覧を Product Decision Ledger の正本とする。

- UX の意味変更: 表示文法、ラベル、badge、並び、grouping、確認 dialog、エラー表示、通知、既定値。
- 振る舞い仕様: 完了、削除、snooze、carry over、overflow など、ユーザー操作の結果や意味。
- データ意味: 集計、永続化、同期、削除、復元、conflict、identity、重複扱い。
- cross-surface 契約: UI / MCP / Shortcuts / notification で同じ操作をどう扱うか。
- QA expectation: 既存 scenario、regression expectation、確認観点が変わるもの。
- プロダクト概念: 用語、概念の意味、このプロジェクトの mission に関わる判断。

報告対象にするのは、現在のユーザー依頼、`backlog/backlog.md`、`docs/rules/` / `docs/specs/` / `docs/decisions/` に明記されておらず、`boundary-control` / `design-decision` / `module-boundary` / `thermo-nuclear-code-quality-review` などの判断系 skill でも実装判断として明確に処理できず、Claude がステークホルダー判断に近い選択をしたものに限る。

報告対象にしないもの:

- backlog や docs に明記済みの内容を、その通り実装しただけのもの。
- 判断系 skill の基準で自動判断できる実装寄りの設計判断。例: module / folder / helper / private API / test structure / 局所 refactor。
- 単なる未実装 TODO や、今回の scope 外の adjacent work。

可逆で影響が小さい選択は採用案で進め、Product Decision Ledger の対象なら ledger に残す。複数の妥当案が残り、かつ選択が非可逆（データ保持・削除・マイグレーション・外部公開契約）またはやり直しコストが大きい場合、または正本と矛盾する場合は、ledger に仮案を書いて押し切らず、呼び出し元 workflow の Stop Conditions に従う。

ledger には次を残す:

- **Trigger**: どのカテゴリに触れたか。
- **Source check**: backlog / docs / decisions / 現在の依頼に明記があるか。
- **Skill check**: 判断系 skill で実装判断として解けるか。
- **採用案**: Claude が選んで進めた案。
- **別案**: 既存挙動に寄せる案と、他の妥当案。
- **理由と影響**: 採用理由、QA / UX / データ意味への影響。
- **ユーザー判断**: Goal 完了報告へ引き継ぐか。引き継がないなら理由。
- **同期先**: 必要なら docs / backlog / decisions のどこへ反映するか。

## What To Record

- **決めたこと**: 何を採用し、何を採用しなかったか。
- **背景**: Goal、backlog、仕様、ユーザー価値のどれに効く判断か。
- **選択肢**: 少なくとも採用案と有力な別案を書く。
- **判断基準**: YAGNI、後工程の手戻り、データ互換、UX、タスク管理、依存方向、テスト容易性など、今回効いた基準を書く。
- **理由**: 原則名ではなく、その選択で何が守られるかを書く。
- **影響**: 後で楽になること、難しくなること、残るリスクを書く。
- **同期先**: 必要なら `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` のどこへ反映すべきかを書く。

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

## Recommended Shape

```md
## 判断: <何を決めたか>

<背景。ユーザー価値、Goal、制約を短く書く。>

別案:
- <別案 A と、その利点 / 問題>
- <別案 B と、その利点 / 問題>

採用:
<採用案。なぜ今回はこちらを優先したか。>

影響:
<後工程、YAGNI、手戻り、未解決リスク、必要な同期先。>
```

## Quality Bar

- 読む人が、コードを読まなくても判断の理由を理解できる。
- 「実装が簡単だったから」ではなく、タスク管理・ユーザー価値・後工程の手戻りに照らして説明できる。
- 採用しなかった案にも、その案が妥当だった理由がある。
- 判断ではないものは削り、必要なら backlog や specs へ移す。
