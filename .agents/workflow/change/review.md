# Review Workflow

## ICAR

- **Intent**: 完了前に、差分が要求・仕様・既存設計を壊していないことを確認する。
- **Constraints**:
  - 粗探しではなく、実害・仕様逸脱・テスト不足・設計劣化を見る。
  - 小さい変更は self-check でよい。
  - Goal の Normal 以上は Gatekeeper（Change ごとの fresh context-free `worker`）が、Conductor の不変 Change brief と plan に full diff を照合し、test を再実行し、review lane を起動・統合して最終採否を行う。Small だけは Conductor の直接 diff 照合で省略できるが、照合開始後に想定を超える差分量・複雑さだと分かった場合は、その場で直接照合を打ち切って Gatekeeper 起動へ切り替える。
  - Implementer は review lane、採否、commit を担当しない。Gatekeeper も編集せず、指摘は Conductor 経由で同じ Implementer に差し戻す。単発 Change だけは current agent が差配する。
  - テスト可能な振る舞い変更や bug fix に unit test / regression test がない場合は、原則 blocker として扱う。
  - review 開始前に、commit に含める code / tests / `backlog/backlog.md` / `docs/specs/` / `llm-wiki/` / `docs/decisions/` / ADR の内容変更が完了していることを確認する。未完了なら review せず `change/implement.md` に戻る。
  - product decision（UX・データ意味・cross-surface 等。カテゴリ一覧は同ファイル）を含む差分は、`.agents/workflow/design-decision-record.md` の基準で現在の要求 / backlog / docs / decisions または Product Decision Ledger から採用案・別案・理由を追えることを確認する。追えない場合、または指摘対応で新しい product decision が発生した場合は `change/implement.md` に戻る。
  - 公開 API / 削除 / 並行性 / 永続化 / 広い UI 挙動などは、`change-review` に加えて別視点レビューを使う。<!-- slot: 足す領域固有レビュー観点があれば追記する（例: UI 層に触れるなら対応する specialist skill）。 --><!-- /slot -->
  - 構造劣化リスクがある場合は `thermo-nuclear-code-quality-review` を必ず使う。
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
  - `codex-fresh-review` skill（fresh Review subagent の起動入口）
  - 関連する `docs/rules/`, `docs/specs/`, `llm-wiki/`（作業地図）

## Depth

- **Self-check**: Small 変更。Goal では Conductor、単発では current agent が `git diff` を読み、要求と検証結果を照合する。
- **Standard**: Small 以外の実装差分。Gatekeeper が fresh read-only finder `scout` を review lane として起動し、必要な観点と追加 skill を統合する。
- **Targeted supplement**: 領域固有リスクがある変更。Review subagent が `change-review` に加えて Constraints に挙げた領域固有観点で確認する。構造劣化リスクがある場合は `thermo-nuclear-code-quality-review` を必須とする。
- **External supplement**: 大きい、曖昧、High-risk、または設計判断が重い変更。Review subagent が必要な別視点レビューを入れる。

## Gatekeeper and Finder

- Gatekeeper は `spawn_agent(worker, fork_turns: "none")` の fresh context-free subagent とし、brief と repo state だけから判断する。review lane の finder は `scout` とし、tree-wide active subagent 3 を超えないよう並列数を調整する。
- Gatekeeper は対象の commit 前差分全体、Change brief、plan、関連正本、必要な test を直接読む。Implementer の報告・実装経緯を受け入れ根拠にしない。
- Gatekeeper は `change-review` をレビュー観点として使い、必要な領域固有 skill の観点も利用する。finder は Gatekeeper に finding 候補を返す read-only lane である。
- finder はファイル編集・git 書き込み・ビルド・テストを行わず、実害のある finding または `LGTM` を返す。Gatekeeper は必要な test を再実行する。
- 観点由来の finder に加えて、全列挙観点の finder を常設で 1 体起動する。担当: diff が追加・変更した型・フィールド・enum ケースごとに、リポジトリ横断で全構築・全 read/write・全変換サイトを file:line で列挙し、1 箇所ずつ変更の反映を確認する。同じ意味論の変換関数が別経路に同型で存在しないかを、既存の兄弟フィールド・兄弟関数の消費サイトからの逆引きで探すことまで含める（欠落型欠陥＝diff 外の反映漏れは diff 実読と通常観点では検出できないことが実測されているため、この観点だけ列挙を義務化する）。subagent 枠が足りない場合も、この観点は省略せず Gatekeeper 自身が列挙を実行する。
- Gatekeeper が finding の採否を行い、Conductor が同じ Implementer に修正・再検証を依頼する。return 後の差分は Gatekeeper が full diff と証拠を再照合してから accept する。Conductor だけが commit する。
- 必要な lane が実行不能な場合は review 済みにせず停止する。review lane は `send_message` で running agent に問い合わせ、completed / idle agent は `followup_task` で再開する。
- 追加調査や観点分割が有効で、agent depth budget が許す場合だけ、Gatekeeper は finder を起動して結果を統合してよい。depth budget が足りない場合は、Gatekeeper 自身で確認するか、Implementer に戻して委任方針を切り替える。
- reviewer 数、観点数、再レビュー回数は固定しない。

## Maintenance Findings

通常 review では maintenance-audit へ自動遷移しない。今回の差分を超える構造劣化・backlog 整理・ドキュメント整合性問題を見つけた場合は、今回の blocker でない限り別タスクとして `backlog/backlog.md` または `maintenance.md` の対象に切り出す。review 対象範囲内の問題の検出・報告は active scope だが、その修正の着手は `change/workflow.md` の横断スコープ制御で分類する（差分内の blocker は workflow-required、差分を超える改善は adjacent として capture / report）。

## Goal Boundary

この review は 1 commit / Change の commit 前差分だけを対象にする。Goal range では `goal.md` に従い、実行直前に固定した `<review_cursor>..<review_end>` への Goal Review（fresh Codex review。ユーザー明示時は Claude review も）だけを行い、ここでの Self Review / `change-review` を再実行しない。

Goal 経由の Change Review は局所的な correctness / spec / tests を担当し、commit 間の統合、Goal Acceptance、構築・read / write site の貫通、docs / backlog 整合は Goal Review が担当する。Goal を経由しない単発 Change では、Change Review がこれらの貫通・整合も担当する。

## Stop Conditions

- 指摘対応中に `change/workflow.md` の判断境界で Stop に該当する仕様・UX・設計方針が発生した。
- Gatekeeper または必要な review lane が実行できない。
