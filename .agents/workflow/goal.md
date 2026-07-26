# Goal Workflow

この workflow はこのプロジェクトの Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

## ICAR

- **Intent**: `/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。
- **Constraints**:
  - 役割は次の 3 層に固定し、判定が割れたときだけ Advisor を、停止報告の前だけ Auditor を足す: **Conductor**（main。Change brief・commit slicing・agent 起動・機械照合・commit・Goal Review・最終報告。実装せず、Small の明示的例外以外は per-commit の詳細 diff / test log / review 往復を読まない）、**Implementer**（Change ごとの fresh `worker`。plan・実装・検証のみ）、**Gatekeeper**（Change ごとの fresh・実装文脈なし `worker`。full diff、brief / plan、再実行 test、review lane、acceptance を担当し、編集しない）、**Advisor**（常設ではなく、判定が割れたときだけ Conductor が呼ぶ読み取り専用の fresh agent。発火条件・禁止事項は `Advisor` 節を正とする）、**Auditor**（常設ではなく、return 上限到達で未解決 MUST が残り停止報告する前だけ Conductor が呼ぶ読み取り専用の fresh agent。当事者の説明を渡さず証拠だけで続行 / 縮小 / 破棄を推奨する。発火条件・入力制限は `Advisor` 節の Auditor 規定を正とする）。各役割の既定モデルと effort は `models.md` を正とする。
  - 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
  - `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal $goal-workflow に従い、backlog/backlog.md の「v0.x」を完了して。`
  - Goal 開始時の `HEAD` を base commit として記録する。`base` は Goal 終了まで動かさず、Goal 全体の差分 `<base>..HEAD` と最終報告の起点にする。分割レビューの進捗は `review_cursor`（初期値 `base`）で別に持つ。ブランチは切らず、range で対象を表す。
  - 1 回の実装 workflow は 1 commit 単位に限る。Conductor は実装を直接担当せず、Goal が 1 commit だけで完了する場合も、次の 1 Change を選んで fresh Implementer を 1 つずつ直列起動する。Normal 以上は fresh Gatekeeper の accept 後にのみ Conductor が commit する。
  - 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
  - Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
  - 複数案の判断は `change/workflow.md` の境界に従う。可逆で影響が小さい選択は採用案で進め、複数の妥当案が残り、かつ選択が非可逆またはやり直しコストが大きい場合、または正本と矛盾する場合は Stop Conditions に従う。
  - 仕様・UX の不明点は `change/workflow.md` の判断境界に従う。Product Decision Ledger の対象・記録・報告基準は `.agents/workflow/design-decision-record.md` を正本とし、Goal 完了時は各 Change の ledger、review 結果、同期済み docs から `ユーザー判断が必要` の有無を集約する。
  - 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
  - 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
  - workflow の review とは別に、commit 済み range への Goal Review（`Goal Review` 節）を Goal 完了条件に含める。Goal range に Change ごとの Standard Change Review を再実行しない。
  - Gatekeeper の return は最大 2 往復。未解決 MUST が残れば commit せず停止する（停止報告の前に `Advisor` 節の Auditor を起動し、その推奨を添えて報告する）。SHOULD 以下だけなら、Gatekeeper が具体的な残リスクを明示受容した時だけ accept できる。
  - 互いに独立した read-only 調査・レビューは並列化してよい。調査は `scout`、writer は `worker` とし、同一 worktree の writer は 1 体に限る。running agent には状態を照会し、completed / idle agent は同じ agent に追加作業を渡して再開する。
- **Acceptance**:
  - Goal の目的が満たされている。
  - 必要な commit がすべて作成されている。
  - 各 commit が `change/workflow.md` の workflow を満たしている。
  - Goal 開始時 base 以降の commit 済み内容が Goal Review 済み。レビュー上限に到達した場合は、最終修正が未レビューであることを含めて `レビュー上限超過` として報告されている。
  - 必要な仕様・backlog・判断記録が同期されている。
  - ユーザー判断が必要な項目の有無が、各 Change から引き継いだ記録に基づいて完了時に明示されている。
  - 作業ツリーの残差分がない、または残す理由が明確。
- **Relevant**:
  - `goal-workflow` skill
  - `.agents/workflow/change/workflow.md`
  - `.agents/workflow/goal-review.md`（Goal Review の実行手順の正本。実施直前に読む）
  - `.agents/workflow/design-decision-record.md`
  - `models.md`（モデルの短名・序列・reasoning effort・役割既定の正本）
  - `backlog/backlog.md`
  - 関連する `docs/rules/`, `docs/specs/`, `docs/decisions/`, `llm-wiki/`（作業地図）

## Flow

1. Goal の目的、制約、完了条件を確認し、開始時の base commit を記録する。ブランチは切らない。
2. Goal を 1 commit 単位の候補へ分割する。
3. 次に扱う 1 commit 分の Change brief（scope・変更対象領域〈ファイルまたはディレクトリ・モジュール単位の具体列挙。Gatekeeper の brief 外ファイル照合の基準になる〉・Acceptance・非対象・設計制約・一時 artifact 置き場 `tmp/workflow/<scope>/`。`<scope>` は Goal の短い slug〈例: `v0110_1`〉で Goal 全体で固定）を確定し、fresh Implementer に同じ brief だけを渡す。Goal が 1 commit だけの場合も同じ。brief は Conductor が確定した最新版が唯一の版であり、Gatekeeper の brief 外ファイル照合で正当な波及と判定して brief を更新した場合は、その更新後の版を Implementer / Gatekeeper 双方の以降のやり取りで同じ最新版として扱う（版の途中の内容を Conductor 以外が書き換えない）。
4. Small は Conductor が直接 diff を照合する。ただし、照合を始めた後に Small の想定を超える差分量・複雑さだと分かった場合は、その場で直接照合を打ち切り、fresh Gatekeeper の起動へ切り替える。Normal 以上は fresh Gatekeeper が full diff、brief / plan、test 再実行、review lane を照合して accept / return を判定する。return は Conductor 経由で同じ Implementer にまとめて渡し、修正・再検証後は必ず同じ Gatekeeper が diff / evidence を再照合してから accept する。
5. accept 後、Conductor は baseline SHA、status、diff stat / hash、test exit code の機械照合だけを行い、test 後も status / hash を再照合して commit する。
6. 残りがあれば次の 1 commit 分に戻る。必要な Goal Review と対応が済んでいなければ実施する。
7. Goal Review MUST は、review lane の見逃し / テスト設計・再実行の不足 / diff・plan 照合の見逃し / handoff・Acceptance の欠落 / Gatekeeper 通過後の差分変異 / Change 単体では検出不能な Goal 統合問題 / 判定不能、の 7 分類で記録する。
8. 完了または停止する時は、Goal 全体の結果、残リスク、ユーザー判断が必要な項目、レビュー上限超過の有無をまとめる。停止時は停止理由と解決すべきことが分かるようにする。

## Commit Slicing

- 1 commit に独立した複数作業を混ぜない。
- 1 commit は、単独で説明できるユーザー価値、仕様同期、リファクタ、テスト追加のいずれかに寄せる。
- 仕様同期と実装は、同じ変更の理解に必要なら同じ commit に含めてよい。
- 広いリファクタと振る舞い変更は、レビューしづらくなるなら分ける。
- 途中で 1 commit として不自然になったら、作業を広げず commit 単位を切り直す。
- Goal に必要な残作業は、次の Change として続けるか、別タスクが適切なら `backlog/backlog.md` に残す。どちらの場合も漏らさない。

## Implementer

- Goal 経由の Change は、commit 数に関わらず、fresh Implementer session に渡す。
- fresh Implementer は full-history fork ではなく新規コンテキストで起動する。前 commit / 前 Implementer の会話履歴は渡さず、必要な事実は commit、差分、現在のファイル、backlog、review result など現在の repository state から確認させる。
- Conductor は実装を直接担当しない。責務は base / review_cursor 管理、commit slicing、brief 確定、agent 起動、機械照合、commit、Goal Review、最終報告に限る。
- Conductor は次の 1 Change を選び、fresh Implementer を 1 つずつ直列起動する。同じ worktree で複数の Implementer を並行実行しない。Gatekeeper も brief と repository state 以外の実装文脈を引き継がない fresh worker とする。
- Implementer の既定は `worker` Custom Agent（短名・effort の既定は `models.md` を正とする）。ユーザーは Goal の呼び出し文で `implementer: <短名> [effort], gatekeeper: <短名> [effort]` の形でモデルと reasoning effort を役割ごとに明示指定できる（短名→フル ID・effort の解決は `models.md` の表を正とする。無指定の役割は既定のまま）。Codex からは GPT 系のみ起動できるため、Claude 系の短名（`fable` / `opus` / `sonnet` / `haiku`）を指定されたら停止してユーザーに確認する。明示指定された役割だけ `worker` を使わず、`model` と `reasoning_effort` を直接指定して fresh subagent を起動する。難度や利用可否を理由に別モデルへ暗黙 fallback しない。指定は原則 Goal 全体で固定し、Change 単位で黙って差し替えない（High-risk での引き上げは次項の条項に従う）。Conductor / Advisor / Auditor は `models.md` の既定固定で、Goal 呼び出し文からは指定しない。
- reasoning effort は既定（`models.md`）のまま動かさない。High-risk Change で、文脈を十分与えても誤る「問題が難しい」型の失敗が観測された場合に限り、Conductor は同系統の 1 段上のモデル（`models.md` の序列）への引き上げを検討してよい（既定は引き上げなし。実施したら最終報告に理由と結果を記録する。系統は跨がない）。読み飛ばし・検証不足型の失敗は、引き上げでなく契約項目と差し戻しで直す。この判断軸は「既定 effort で明確に試みても誤るならモデルのサイン」という区別であり、それ以外では難度を理由に引き上げない。ユーザーの明示指定（モデル・effort とも）が常に最優先。
- Conductor は会話履歴を引き継がない fresh subagent を起動する。既定では `worker` を使い、ユーザーがモデルを明示した役割は指定されたモデルと effort で起動する。起動結果と rollout / usage の実モデルが指定と一致しなければ、その結果を採用せず停止する。
- Implementer の完了を確認してから次へ進む。追加指示は、running agent には送信し、completed / idle agent には同じ agent を再開して渡す。中断と状態確認も利用し、完了・中断を確認するまで別 writer を起動しない。
- Implementer は渡された Change だけを active scope とし、Goal 全体を再計画・再分割しない。
- 通常は `change/workflow.md` に従い、調査から実装・検証まで完了して戻る。commit と review lane は担当しない。
- 1 commit として不自然だと分かった場合は、作業を広げず事実を Conductor に返す。Conductor が commit 単位を切り直す。
- 戻りの表示形式は固定しないが、終了種別（completed / stopped / blocked / interrupted のいずれか）、plan 参照、変更ファイル、検証コマンドと結果、逸脱・自己判断、commit message 案を必ず引き継ぐ。stopped / blocked の場合は理由と判断点を含める。
- Implementer が session / turn 上限、完了通知待ち、または未完了 handoff で止まった場合は、fresh 再起動より先に同じ agent を再開する。再開条件と fresh recovery への切替条件は「Subagent Progress and Recovery」を正とする。
- Implementer / Gatekeeper / subagent の報告どうしが食い違う場合、Conductor はどれかを採用する前に実ソース・実測で裏取りしてから記録・報告する。裏取りで決着しない場合だけ Advisor に相談する（`Advisor` 節）。
- 直接実行の例外は Goal 経由の作業には適用しない。Goal を経由しない単発 Change だけは、現在の agent が直接実行してよい。
- Implementer は、独立委任が効率または品質を高める調査・実装補助・検証を必要に応じて下位 subagent に任せてよい。下位 subagent のモデルは Implementer モデル保証の対象外。

## Gatekeeper

- Gatekeeper の存在理由は、(1) finding の採否を実装文脈から独立させる、(2) full diff 読解・brief / plan 照合・test 再実行を必ず通す手続き上の実行点を作る、(3) per-commit の詳細を Conductor の複利コンテキストから隔離する、の 3 点である。
- Gatekeeper の既定モデルは上げない（モデル階級を上げてレビュー検出力が上がった実測がない）。Implementer の High-risk 引き上げとは意図的な非対称。
- Gatekeeper は直接修正を一切しない。docs / typo 級を含む全修正は Conductor 経由で Implementer に return する。自分の修正を自分で受け入れないことで、accept 後の diff が変異しない照合前提を守る。
- **brief 外ファイルの照合**: commit に含める予定の全ファイル（tracked の変更・untracked の新規追加を含む）のうち、Change brief の変更対象領域に含まれないものがあれば、内容の正誤に関わらず scope 超過として return する（通常の return と同じく上限 2 往復にカウントする）。判定は「ファイルが brief の変更対象領域に含まれるか」の機械的な照合であり、修正の必要性の意味判断はしない。正当な波及だった場合は、Conductor が brief の変更対象領域を更新してから同じ Implementer に再提出させる。brief 更新は Final Report に記録する。上限に達しても scope 超過が未解消（brief 外ファイルが残る）の場合は、未解決の MUST と同等に扱い、SHOULD 以下の残リスク受容経路には流さない。
- 戻りには次の実行証拠を必須とする: baseline HEAD SHA、`git status --porcelain` の対象状態、commit 予定 diff 全体の hash と stat（hash の算出式は `change/finish.md` を正とする）、test command / exit code / duration（成功時はこれのみ、失敗時だけ output tail）、commit に含める予定の全ファイル（tracked の変更・untracked の新規追加を含む）のうち Change brief の変更対象領域に含まれないものの一覧（なければ「なし」）、起動した review lanes、accept / return 判定、採用した finding と return 要求、後続 Change への影響事実、残存 risk、Product Decision Ledger 候補。宣言だけの報告は受理しない。

## Advisor

判定が割れて Conductor の機械的裏取りでは決着しないときに、方向を決めるためだけに呼ぶ役割。常設の工程ではない。

- 実体は読み取り専用の fresh agent（モデルは `models.md` の役割既定。Codex 側は `sol` 固定）。Conductor が起動し、割れている論点・両者の主張・Conductor が実施した裏取りの結果・関連する実ファイルのパスを渡す。
- **発火条件はこの 2 つだけ**。これ以外では呼ばない:
  - (a) Implementer と Gatekeeper の主張が割れ、Conductor の機械的裏取り（`git show`、実ソースの読み取り）で決着しない。
  - (b) Goal Review と per-Change Gatekeeper の判定が割れ、Conductor の機械的裏取り（該当 commit の `git show`、実ソースの読み取り）で決着しない。
- 返すのは方向の判定と根拠だけ。ファイルを書かない・修正案を実装しない（実装者にしない）。tracked file の編集、git 書き込み、テスト以外の副作用のあるコマンドを禁じる。
- Advisor の相談は差し戻しの往復にカウントしない（Gatekeeper の return 上限 2 往復、Goal Review の reviewer 呼び出し上限 3 回のどちらにも数えない）。
- Advisor の判定は Conductor の判断材料であって、受け入れ判定の代行ではない。採否・commit の責任は従来どおり Gatekeeper と Conductor に残る。判定が Stop Conditions に該当する product decision に触れる場合は、Advisor の結論があっても停止してユーザーに確認する。

Advisor とは別役割として **Auditor** を置く。Advisor は当事者が自分の文脈を説明して助言を受ける相談席であり、Auditor は当事者の説明を遮断し証拠だけで評価する監査席で、同居させない。

- 発火条件は、Gatekeeper の return 上限（2 往復）に達しても未解決の MUST が残り、Conductor が停止報告する場合だけ（`Constraints` の return 上限規定参照）。Conductor は停止報告の前に Auditor を起動する。
- 実体は読み取り専用の fresh agent（モデルは `models.md` の役割既定）。差分・ファイル・git 状態を一切変更しない（監査対象を自分で変えないため）。Implementer / Gatekeeper / Conductor の会話文脈・説明・言い分は一切渡さない。当事者の説明を聞くと判定が汚染されるため、これが Auditor の生命線。
- 渡すのは Change brief、git の生データ（HEAD、diff stat、変更ファイル一覧、diff 本文〈全文。続行/縮小/破棄の判定は diff 本文なしでは根拠づけられないため、大きい場合も stat だけで済ませない〉）、正本（backlog の該当項目、関連する ADR / specs）のみ。
- 出力は「続行 / 縮小 / 破棄して再設計」のいずれかの推奨と根拠。続行 = 現差分のまま進める（膨張は正当な波及）。縮小 = 差分の一部だけ残し、残りは別 Change へ切り直すか捨てる。破棄 = 未コミット差分を捨て、得られた知見（発見したバグ・必要な不変条件・再現テスト）だけを列挙して持ち帰り、fresh Implementer が最小設計で組み直す。
- Auditor の推奨は拘束しない。決定は常にユーザー。Conductor は Auditor の推奨を添えて停止報告する。
- Auditor を起動できない場合は、停止自体は妨げない。停止報告に「Auditor 推奨なし（起動不可）」と明示してそのまま停止報告する（Advisor の発火条件不在時と異なり、Auditor 起動不可は新たな Stop Conditions の対象にしない）。

## Subagent Progress and Recovery

- Conductor 側の待機 timeout は polling window にすぎず、subagent の失敗・終了・session / turn 上限を意味しない。Goal workflow は固定の経過時間だけで agent を巻き取らない。
- running agent への polling が 2 回連続で timeout したら、同じ agent へ status request を送り、現在 phase / 実行中コマンド / 直近の実質進捗 / 残作業 / blocker を求める。既知の長時間コマンドが動いている、または応答に実質進捗がある場合は同じ agent を継続して待ち、fresh recovery を起動しない。コマンド状態が変わるまで同じ status request を反復しない。
- running agent が status request にも応答しない場合だけ、明示的に interrupt する。終了を確認するまで別 writer を起動しない。終了確認後、Conductor は `git status --short`、`git diff --stat`、必要な `git diff`、`git log --oneline -n` で実状態を確認する。
- completed / idle agent が Acceptance 未達、session / turn 上限、時間切迫、または未完了 handoff を返した場合、待機では続行しないが、fresh agent より先に同じ agent を再開する。残作業と優先順位だけを渡し、既に完了した調査・実装・検証を再実行させない。
- fresh recovery へ切り替えるのは、同じ agent を再開できない、agent の文脈または差分が今回の Change と整合しない、または同じ agent の再開 2 turn で実質進捗が観測できない場合に限る。実質進捗は、scope 内の diff / artifact の変化、新しい test / build / review evidence、または根拠付きで blocker・残作業が縮小したことのいずれかで判断し、status 文面の更新だけでは進捗とみなさない。単なる polling timeout、自己申告の時間切迫、正常な未完了 handoff は fresh recovery の条件にしない。
- stopped / blocked は未完了 handoff と区別し、同じ入力のまま自動再開しない。Stop Conditions または不足する判断を解消してから再開する。
- recovery が必要な場合、元 subagent の終了を確認し、未コミット差分が今回の Change scope 内にあると判断できた後だけ、元が Implementer なら fresh recovery Implementer、元が Gatekeeper なら fresh recovery Gatekeeper に同じ担当作業を続行させる。Conductor は実状態の確認と引き継ぎに留め、実装・受け入れ判定を代行しない。差分が scope 外、破壊的、または完了状態を判断できない場合は、差分を破棄せず停止してユーザー確認する。
- この回収手順は例外処理であり、通常の Implementer / Gatekeeper に定期報告ファイルや常時 ledger を要求しない。

## Final Report

- 完了時も停止時も、報告形式は状況に合わせて分かりやすく整える。固定テンプレートに無理に合わせない。
- `ユーザー判断が必要: なし` または必要な判断内容を必ず明示する。
- `ユーザー判断が必要` は `.agents/workflow/design-decision-record.md` の基準で、各 Change の ledger、review 結果、同期済み docs から判断する。記憶だけで `なし` と判断しない。
- Goal Review の結果（各ラウンドの range・実際に呼び出した reviewer skill・verdict・指摘件数）は `tmp/workflow/<scope>/goal-review-log.md` から転記する。記憶で再構成しない。
- `レビュー上限超過: なし` または対象単位・回数・最後の指摘・行った修正・最終修正が未レビューであること・残リスクを状況に合わせて明示する。収束した review も、どの review が通ったかを状況に合わせて報告する。
- 停止時は、停止理由と解決すべきことが分かるようにする。

## Goal Review

- Goal Review は、通常の `change/review.md` とは別に Goal 完了条件として扱う。
- Change Review は個々の commit の局所的な correctness / spec / tests を見る。Goal Review は commit 間の統合、Goal Acceptance、構築・read / write site の貫通、docs / backlog 整合を見る。
- 各 commit の Change Review は `change/review.md` で完了済みとして扱い、Goal range に対して Standard Change Review は再実行しない。
- **実行手順の正本は `goal-review.md`**: Goal Review を実施する直前に毎回 `goal-review.md` を読み、その内容だけに従って reviewer を起動する。この節が持つのは実施タイミングの規定だけ。
- レビュー対象は未コミット差分ではなく、未レビュー範囲の commit range とする（分割しない場合は `review_cursor == base`）。ブランチは切らないので range で対象を表す。
- **分割レビュー**: 一気に全部ではなく、適当なコミットのまとまりごとにレビューしてよい（毎回でなくてよい）。1 commit ごとではなく、関連する数 commit をまとめてレビューする。PASS 相当なら `review_cursor` を `review_end` まで進める（`base` は動かさない）。次のレビューは新しい `review_cursor` から、実行直前に新しい終点 SHA を取り直す。
- 差分が大きい、または永続化 / 同期 / 外部 API / 広い UI 挙動に触れる場合は、数 commit を待たずにその時点までの range で早めにレビューする。
- Goal 完了までに、`base` から Goal の最終 commit までの全 range がレビュー済みであること（通過条件は `goal-review.md`）。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が `change/workflow.md` の判断境界で Stop に該当する重要な仕様・UX・データ保持・削除方針に依存している。
- Goal の途中で、現在の目的と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- 必須の検証を代替手段でも裏付けられず、完了扱いにできない。
- Goal Review の 2 本の reviewer のどちらかを完全に実施できない。
- Goal Review の 2 本の reviewer で指摘が割れ、機械的な合流では解けない。
- 指定された Implementer agent type / モデルを実測可能な起動経路で保証できない、または rollout の実モデルが指定と一致しない。
- Goal の呼び出し文で Implementer / Gatekeeper に Claude 系の短名が指定された（Codex からは GPT 系しか起動できない）。
- Advisor の発火条件に該当したが、Advisor を起動できない。
