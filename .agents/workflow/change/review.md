# Review Workflow

## ICAR

- **Intent**: 完了前に、差分が要求・仕様・既存設計を壊していないことを確認する。
- **Constraints**:
  - 指摘の採否は、粗探しでなく実害・仕様逸脱・テスト不足・設計劣化を基準にする。この絞り込みは Gatekeeper の採否段で行い、finder 段には課さない（`Gatekeeper and Review Lanes` 参照）。
  - 小さい変更は self-check でよい。
  - Goal の Normal 以上は Gatekeeper（Change ごとの fresh context-free `worker`）が、Conductor の不変 Change brief と plan に full diff を照合し、test を再実行し、review lane を起動・統合して最終採否を行う。Small だけは Conductor の直接 diff 照合で省略できるが、照合開始後に想定を超える差分量・複雑さだと分かった場合は、その場で直接照合を打ち切って Gatekeeper 起動へ切り替える。
  - Implementer は review lane、採否、commit を担当しない。Gatekeeper も編集せず、指摘は Conductor 経由で同じ Implementer に差し戻す。単発 Change だけは current agent が差配する。
  - Standard の一般レビューには fresh な standard-review-coordinator 経由で global `diff-review` skill を使い、別の一般レビュー skill は重ねない。
  - テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合は、原則 blocker として扱う。
  - review 開始前に、commit に含める code / tests / `backlog/backlog.md` / `docs/specs/` / `llm-wiki/` / `docs/decisions/` / ADR の内容変更が完了していることを確認する。未完了なら review せず `change/implement.md` に戻る。
  - product decision（振る舞い仕様が変わる判断。定義は同ファイル）を含む差分は、`.agents/workflow/design-decision-record.md` の基準で現在の要求 / backlog / docs / decisions または Product Decision Ledger から採用案・別案・理由を追えることを確認する。追えない場合、または指摘対応で新しい product decision が発生した場合は `change/implement.md` に戻る。
  - 公開 API / 削除 / 並行性 / 永続化 / 広い UI 挙動などの High-risk 変更は `$diff-review xhigh` を使う。領域固有 skill がある場合は追加する。<!-- slot: 足す領域固有レビュー観点があれば追記する（例: UI 層に触れるなら対応する specialist skill）。 --><!-- /slot -->
  - 構造劣化リスクがある場合は `thermo-nuclear-code-quality-review` を使う。
  - 指摘に対応しない場合は理由を残す。
  - Gatekeeper と review lane はファイルを変更しない。採用 finding は一回の return にまとめ、最大 2 往復後にも MUST が残れば停止する。SHOULD 以下だけは Gatekeeper の具体的な残リスク受容時のみ accept できる。
  - review は commit 前の局所品質ゲートであり、最終保証ではない。採用した指摘を修正した後に再レビューするかは、差分の大きさ、risk、MUST 指摘の内容、新しい設計判断の有無から判断する。
  - 修正後に再レビューしない場合も、対応しない指摘・残リスク・Goal Review で見るべき観点があれば記録する。`レビュー上限超過` は Change 内 review では使わない。
- **Acceptance**:
  - 選んだレビュー深度と理由が説明できる。
  - review 対象が commit 予定差分全体（code / tests / docs / `backlog/backlog.md` / `docs/decisions/` を含む）である。
  - product decision を含む差分では、報告対象と報告不要な実装判断が分かれている。
  - 指摘があれば対応済み、または対応しない理由が明確。
  - レビュー後に変更した場合、必要な再検証が済んでいる。
- **Relevant**:
  - 変更差分
  - plan または要求
  - 検証結果
  - global `diff-review` skill
  - 関連する `docs/rules/`, `docs/specs/`, `llm-wiki/`（作業地図）

## Depth

- **Self-check**: Small 変更。Goal では Conductor、単発では current agent が `git diff` を読み、要求と検証結果を照合する。
- **Standard**: Small 以外の通常リスクの実装差分。Gatekeeper が fresh な standard-review-coordinator を起動し、coordinator が `$diff-review high` を実行する。High-risk のみ `$diff-review xhigh` に上げる。
- **Targeted supplement**: High-risk 変更は Standard の代わりに `$diff-review xhigh` を使い、該当する危険領域だけ独立 pass を追加する。領域固有 skill がある場合は併用する（構造劣化リスク時の `thermo-nuclear-code-quality-review` を含む）。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。Review subagent が必要な別視点レビューを入れる。

## Gatekeeper and Review Lanes

- Gatekeeper は会話履歴を引き継がない fresh な `worker` subagent とし、brief と repo state だけから判断する。補助 lane の finder は `scout` とする。
- Gatekeeper は対象の commit 前差分全体、Change brief、plan、関連正本、必要な test を直接読む。Implementer の報告・実装経緯を受け入れ根拠にしない。
- Standard では Gatekeeper が fresh な standard-review-coordinator を起動する。coordinator は commit 予定の staged / unstaged / untracked 差分全体を対象に `$diff-review high`、High-risk なら `$diff-review xhigh` を実行する。Gatekeeper には整理済み finding または `LGTM` だけを返し、skill の内部 context や finder の中間結果を返さない。
- `$diff-review high` は、追加・変更された型・field・enum case の全構築、全 read/write、全変換 site を横断確認する全列挙 finder を含む。`xhigh` も `high` を内包するため同じ確認を行う。
- `$diff-review xhigh` は `high` の基準を内包するため両方を重ねない。nested coordinator 内の独立 finder は最大 3 体を同時起動し、それを超える観点は batch に分ける。`max_depth = 4` のうち Conductor 0 → Gatekeeper 1 → coordinator 2 → finder 3 までを通常経路に使い、depth 4 は予備として残す。
- `diff-review` の finding は候補であり、Gatekeeper が実際の diff・brief・plan・正本に照らして採否する。coordinator、skill、または必要な参照ファイルが利用不能な場合は review 済みにせず停止する。
- finder / coordinator 段の責務は網羅とする。見つけた問題は低 severity・低確信でも省かず、severity と確信度を添えて全件報告させ、重要度・確信度による自己フィルタをさせない。フィルタ・採否は Gatekeeper 段の責務として分ける（finding 段に重要度の自己フィルタを課すと、調査で見つけた問題を報告段で落とす挙動がベンダー公式ガイドに明記されているため）。
- Targeted / External supplement の finder はファイル編集・git 書き込み・ビルド・テストを行わず、finding（severity・確信度付き）または `LGTM` を返す。Gatekeeper は必要な test を再実行する。
- Gatekeeper が finding の採否を行い、Conductor が同じ Implementer に修正・再検証を依頼する。return 後の差分は Gatekeeper が full diff と証拠を再照合してから accept する。Conductor だけが commit する。
- 必要な lane が実行不能な場合は review 済みにせず停止する。running agent には問い合わせ、completed / idle agent は同じ agent を再開して扱う。
- 追加調査や観点分割が有効で、agent depth budget が許す場合だけ、Gatekeeper は finder を起動して結果を統合してよい。depth budget が足りない場合は、Gatekeeper 自身で確認するか、Implementer に戻して委任方針を切り替える。
- reviewer 数、観点数、再レビュー回数は固定しない。

### standard-review-coordinator

- 会話履歴を引き継がない fresh な `scout` subagent として起動し、現在のレビュー対象と effort（`high` / `xhigh`）だけを受け取る。Change brief、plan、Implementer の報告は渡さず、採否判断も行わない。
- repository root で指定 effort の global `$diff-review` を適用する。skill 内部の finder、候補検証、中間出力は coordinator の context 内で完結させる。
- `$diff-review` の手順ファイルには件数上限（severity 順の切り詰め）や確信度による切り捨ての規定があるが、この workflow から使う場合は網羅責務を報告契約として優先する。coordinator は `$diff-review` 実行時にこの契約（全件報告・severity と確信度の付与・件数上限なし）を明示して渡す。
- Gatekeeper には、整理済み finding（file:line、問題、failure scenario、severity・確信度、推奨対応）または `LGTM` だけを返す。ファイルは変更しない。
- Gatekeeper は coordinator の完了結果をまとめて受け取ってから採否する。running agent には追加連絡し、completed / idle agent は同じ agent を再開して再利用する。

## Maintenance Findings

今回の差分ではなく、複数タスク後の全体構造・負債を見る棚卸しは、ユーザー起点で global `maintenance-audit` skill により行う。`llm-wiki/` を持つ project では、棚卸し結果の反映（修正 Change 群）が済んだ後に global `wiki-lint` で地図（孤立・リンク切れ・sources 切れ）を点検する。通常 review では maintenance-audit へ自動遷移しない。今回の差分を超える構造劣化・backlog 整理・ドキュメント整合性問題を見つけた場合は、今回の blocker でない限り別タスクとして `backlog/backlog.md` へ切り出す。review 対象範囲内の問題の検出・報告は active scope だが、その修正の着手は `change/workflow.md` の横断スコープ制御で分類する（差分内の blocker は workflow-required、差分を超える改善は adjacent として capture / report）。

## Goal Boundary

この review は 1 commit / Change の commit 前差分だけを対象にする。Goal range では `goal.md` に従い、実行直前に固定した `<review_cursor>..<review_end>` への Goal Review（`goal.md` の reviewer 規定に従う fresh reviewer 2 本）だけを行い、ここでの Standard Change Review を再実行しない。

Goal 経由の Change Review は局所的な correctness / spec / tests を担当し、commit 間の統合、Goal Acceptance、構築・read / write site の貫通、docs / backlog 整合は Goal Review が担当する。Goal を経由しない単発 Change では、Change Review がこれらの貫通・整合も担当する。

## Stop Conditions

- 指摘対応中に `change/workflow.md` の判断境界で Stop に該当する仕様・UX・設計方針が発生した。
- Gatekeeper、`diff-review`、または必要な review lane が実行できない。
