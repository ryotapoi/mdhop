# Goal Workflow

この workflow はこのプロジェクトの Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

## Intent

`/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。

## Constraints

- 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
- `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal goal-workflow skill に従い、backlog/backlog.md の「v0.x」を完了して。`
- 役割は次の 3 層に固定し、判定が割れたときだけ Advisor を、差し戻し上限到達の停止報告前だけ Auditor を足す:
  - **Conductor**（この workflow を進める main セッション。commit slicing・次 Change 選定・Change brief 確定・subagent 起動・機械照合・commit 実行・Goal Review 手配・停止判断・最終報告を担う。実装は書かず、per-commit の詳細（diff 本文、テストログ、review 往復）も読まない。受け取るのは Implementer / Gatekeeper からの構造化要約だけで、実物の diff を読むのは報告に食い違い・疑義がある例外時に限る。Small での直接 diff 照合は、この不変条件の明示的な例外である。詳細は Gatekeeper 節参照）。
  - **Implementer**（Change ごとに fresh subagent。計画・実装・検証を担う。commit はしない。review lane は回さない）。
  - **Gatekeeper**（Change ごとに fresh subagent、実装文脈を引き継がない。diff 実読・Change brief / plan との照合・テスト再実行による裏取り・review lane の起動と統合・指摘の採否・受け入れ判定を担う。詳細は Gatekeeper 節を参照）。
  - **Advisor**（常設の役割ではなく、判定が割れたときだけ Conductor が呼ぶ読み取り専用の fresh subagent。発火条件・禁止事項は Advisor 節を正とする）。
  - **Auditor**（常設の役割ではなく、差し戻し上限に達して未解決の MUST が残り停止報告する前だけ Conductor が呼ぶ、読み取り専用の fresh subagent。当事者の説明を渡さず証拠だけで続行 / 縮小 / 破棄を推奨する。発火条件・入力制限は Advisor 節内の Auditor 規定を正とする）。
- `/goal` の呼び出し文で Implementer / Gatekeeper のモデルを役割ごとに指定できる（例: `implementer: opus, gatekeeper: sonnet`）。短名の後ろに reasoning effort を添えて役割ごとに明示してもよい（例: `implementer: opus xhigh`）。無指定は両役割とも既定（短名・序列・effort の有効値と既定は `models.md` を正とする）。この workflow が起動できるのは Claude 系モデルだけなので、GPT 系の短名（`luna` / `terra` / `sol`）を指定されたら停止してユーザーに確認する（`.agents/workflow/` 側で Claude 系を指定されたときと対称）。難度や利用可否を理由に別モデルへ暗黙 fallback しない。Implementer は計画と実装を一体で行い、実装前の plan 書き出しは `change/plan.md` に従う。指定は原則 Goal 全体で固定し、Change 単位で黙って差し替えない（High-risk での引き上げは Implementer 節の条項に従う）。Conductor / Advisor / Auditor は `models.md` の既定固定で、Goal 呼び出し文からは指定しない。
- ブランチは切らず、いるブランチ（通常 main）上にそのまま 1 commit ずつ積む。Goal 開始時の `HEAD` を base SHA として記録する（Goal Review の range 起点）。
- 1 回の実装 workflow は 1 commit 単位に限る。Conductor は実装を直接担当せず、Goal が 1 commit だけで完了する場合も、次の 1 Change を選んで fresh subagent を Implementer として 1 つずつ直列起動する。Implementer の完了後は Gatekeeper（Normal 以上）を起動し、通過したら Conductor が機械照合のうえ commit する。このフローはモデル指定に関わらず共通。
- 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
- Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
- Goal 前提では都度のユーザー確認を避け、自動進行する。止まるのは Stop Conditions に該当する場合だけ。
- plan mode（`EnterPlanMode` / `ExitPlanMode`）は使わない。承認待ちが自動進行と噛み合わないため。計画が必要な場合は内部で立ててそのまま実装する。詳細は `change/plan.md`。
- 複数案の判断は `change/workflow.md` の境界に従う。可逆で影響が小さい選択は採用案で進め、複数の妥当案が残り、かつ選択が非可逆またはやり直しコストが大きい場合、または正本と矛盾する場合は Stop Conditions に従う。
- 仕様・UX の不明点は `change/workflow.md` の判断境界に従う。Product Decision Ledger の対象・記録・報告基準は `.claude/workflow/design-decision-record.md` を正本とし、Goal 完了時は各 Change の ledger、review 結果、同期済み docs から `ユーザー判断が必要` の有無を集約する。
- 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
- 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
- 各 commit の Gatekeeper 判定とは別に、Goal の commit range に対する Goal Review を Goal 完了条件に含める（Goal Review 参照）。Goal range に Change ごとの Standard Change Review は再実行しない。
- Goal Review が MUST を出した場合、その欠陥がどのすり抜けに当たるかを Final Report に記録する（すり抜け記録義務）。分類: review lane の見逃し / テスト設計・再実行の不足 / diff・plan 照合の見逃し / handoff・Acceptance の欠落 / Gatekeeper 通過後の差分変異 / Change 単体では検出不能な Goal 統合問題 / 判定不能。

## Acceptance

- Goal の目的が満たされている。
- 必要な commit がすべて作成されている。
- 各 commit が `change/workflow.md` の workflow を満たしている。
- Goal の commit range（`<base>..HEAD`）が Goal Review を通過している。レビュー上限に到達した場合は、最終修正が未レビューであることを含めて `レビュー上限超過` として報告されている。
- 必要な仕様・backlog・判断記録が同期されている。
- ユーザー判断が必要な項目の有無が、各 Change から引き継いだ記録に基づいて完了時に明示されている。
- 作業ツリーの残差分がない、または残す理由が明確。

## Relevant

- `goal-workflow` skill
- `.claude/workflow/change/workflow.md`
- `.claude/workflow/models.md`（モデルの短名・序列・reasoning effort・役割既定の正本）
- `.claude/workflow/change/review.md`（Gatekeeper が起動する review lane）
- `.claude/workflow/goal-review.md`（Goal Review の実行手順の正本。実施直前に Read する）
- `.claude/workflow/design-decision-record.md`
- `design-decision` skill
- `backlog/backlog.md`

## Flow

1. Goal の目的、制約、完了条件、役割ごとのモデル指定（無指定は `models.md` の役割既定）を確認し、ブランチは切らず開始時の `HEAD` を base SHA として記録する。
2. Goal を 1 commit 単位の候補へ分割する（Commit Slicing 参照）。
3. 次に扱う 1 commit 分を選び、Change brief（scope・変更対象領域〈ファイルまたはディレクトリ・モジュール単位の具体列挙。Gatekeeper の brief 外ファイル照合の基準になる〉・Acceptance・非対象・設計制約・一時 artifact 置き場 `tmp/workflow/<scope>/`。`<scope>` は Goal の短い slug〈例: `v0110_1`〉で Goal 全体で固定）を確定して fresh Implementer に渡す（Goal が 1 commit だけの場合も同じ）。モデル指定は Implementer の実体を決めるだけで、起動手順そのものは共通。同じ Change brief は Gatekeeper 起動時にも Conductor が確定した内容をそのまま渡す（Implementer の報告経由で伝えない）。brief は Conductor が確定した最新版が唯一の版であり、Gatekeeper の brief 外ファイル照合で正当な波及と判定して brief を更新した場合は、その更新後の版を Implementer / Gatekeeper 双方の以降のやり取りで同じ最新版として扱う（版の途中の内容を Conductor 以外が書き換えない）。
4. Implementer の完了後、Intake が Small なら Conductor が diff を直接実読して照合する（Gatekeeper 省略）。Normal 以上なら fresh Gatekeeper を起動し、diff 実読・テスト再実行・review lane・受け入れ判定を行わせる。差し戻しがあれば Conductor 経由で同一 Implementer を `SendMessage` で再開させる（上限 2 往復。上限超過時の扱いは差し戻し上限の節を参照）。
5. Gatekeeper が通過（または Small で Conductor が直接照合）したら、Conductor が機械照合（Gatekeeper 報告の baseline HEAD SHA の実在確認、意図しない git 書き込みの有無、diff --stat の一致、commit 予定差分全体のハッシュの再計算・一致確認、テストの自己実行〈exit code のみ確認〉）を行う。テスト自己実行が worktree を変更し得るため、実行後に `git status` と diff hash を再照合してから commit する。
6. commit 後、Goal の残りと Goal Review の実施タイミングを確認する。残りがあれば手順 3 に戻る。
7. 必要な Goal Review と対応が済んでいなければ実施する。実施の直前に `goal-review.md` を Read し、その手順に従う。PASS 相当なら `review_cursor` を `review_end` まで進めてよい（`base` は動かさない）。Goal Review が MUST を出した場合は、すり抜け記録義務に従い該当欠陥がどの Gatekeeper 手続きをすり抜けたかを記録する。
8. 完了または停止する時は、Goal 全体の結果、残リスク、ユーザー判断が必要な項目、レビュー上限超過の有無をまとめる。停止時は停止理由と解決すべきことが分かるようにする。

## Branch

- ブランチは切らない。いるブランチ（通常 main）上にそのまま 1 commit ずつ積む。
- Goal 開始時の `HEAD` を base SHA として記録する。`base` は Goal 終了まで動かさない。Goal 全体の差分は `<base>..HEAD` で表し、最終報告と全体俯瞰の起点になる。
- 分割レビューの進捗は `review_cursor` で持つ。初期値は `base`。レビューが済むたびにレビュー済みの commit まで `review_cursor` を進める。次の分割レビューでは実行直前の `HEAD` を `review_end` として固定し、対象を `<review_cursor>..<review_end>` にする。`base` と `review_cursor` を混同しない（全体差分は常に `base` 起点、未レビュー差分は `review_cursor` 起点）。
- merge 操作はない。Goal 完了後もそのままブランチ上に commit が残る。
- 履歴は線形に保ち、各 commit を単独で revert / bisect できる状態に残す。

## Commit Slicing

- 1 commit に独立した複数作業を混ぜない。
- 1 commit は、単独で説明できるユーザー価値、仕様同期、リファクタ、テスト追加のいずれかに寄せる。
- 仕様同期と実装は、同じ変更の理解に必要なら同じ commit に含めてよい。
- 広いリファクタと振る舞い変更は、レビューしづらくなるなら分ける。リファクタを先に commit してから振る舞い変更を別 commit にする。
- 途中で 1 commit として不自然になったら、作業を広げず commit 単位を切り直す。
- Goal に必要な残作業は、次の Change として続けるか、別タスクが適切なら `backlog/backlog.md` に残す。どちらの場合も漏らさない。

## Implementer

- Implementer の実体は Goal 指定のモデルで決まる（無指定は `models.md` の役割既定。指定できるのは Claude 系短名のみ）。Implementer（Claude subagent）自身が計画・実装する。Implementer は commit せず、review lane も回さない。
- Goal 経由の Change は、commit 数に関わらず、原則 fresh Implementer に渡す。
- Conductor は実装を直接担当しない。Conductor の責務は、base / review_cursor 管理、commit slicing、次の Change 選定、subagent 起動、機械照合、commit 実行、Goal Review 手配、最終報告に限る。
- Conductor は次の 1 Change を選び、Change brief（内容は Flow 手順 3 と同じ）を確定した上で fresh subagent を Implementer として 1 つずつ直列起動する。同じ worktree で複数の Implementer を並行実行しない。Goal が 1 commit だけで完了する場合も Implementer を 1 つ起動する。
- Implementer の起動も、結果を起動呼び出しの戻り値で受け取る同期実行を基本とする。background になった場合は完了通知を待たず、`SendMessage` で能動的に結果を回収する（`change/workflow.md` の Subagent / Skill 参照）。
- モデルと effort は `models.md` の既定で運用し、既定から外れて effort を動かさない（High-risk Change の Implementer effort 既定も `models.md` が定める）。ユーザーの明示指定（モデル・effort とも）が常に最優先で、呼び出し文で effort が明示された役割はその値で起動する。High-risk Change で、文脈を十分与えても誤る「問題が難しい」型の失敗が観測された場合に限り、Conductor は 1 段上のモデル（`models.md` の序列。最上位の場合は引き上げ先なし。引き上げ後の effort も `models.md` の既定に従う）への引き上げを検討してよい（既定は引き上げなし。実施したら最終報告に理由と結果を記録する。Claude 系の範囲を出ない）。読み飛ばし・検証不足など「頑張りが足りない」型の失敗は、引き上げでなく契約項目（全列挙・検証義務）と差し戻しで直す。この判断軸は Anthropic の公式ガイダンス（既定 effort で明確に試みても誤るならモデルのサイン）由来。それ以外では難度を理由に引き上げない。この引き上げは失敗観測後の是正であり、advisor ツールによる予防的相談（下記）とは別枠で併用する。advisor ツール不在の代替として引き上げを使わない。
- advisor ツールが設定されている環境（Claude Code の `advisorModel` 等。subagent は設定を継承する）では、Implementer の起動プロンプトに相談条件を含める: 非自明な設計判断にコミットする前、同じエラー・失敗が繰り返す時、アプローチの変更を検討する時に相談する。自明な作業では呼ばない。これは Implementer が自分の作業中に使う実装精度の向上手段であり、Conductor が判定の割れを解く Advisor 役割（Advisor 節）とは別物。
- High-risk や設計判断の厚い Change では、advisor ツールの相談を厚くする: 相談条件に加えて、実装方針の確定前と完了宣言前の相談を必須と明記する。advisor ツールが使えない環境で Implementer が High-risk Change に当たった場合は、モデルを引き上げて代替せず停止してユーザーに確認する（Stop Conditions 参照）。
- Implementer は渡された Change だけを担当し、Goal 全体を再計画・再分割しない。
- 通常は `change/workflow.md` に従い、調査から実装・検証まで完了して戻る（commit と review lane の起動は含まない）。
- 1 commit として不自然だと分かった場合は、作業を広げず事実を Conductor に返す。Conductor が commit 単位を切り直す。
- 戻りの表示形式は固定しないが、次を必ず引き継ぐ: 終了種別（completed / stopped / blocked / interrupted のいずれか）、plan 参照、変更ファイル一覧、実行した検証コマンドと結果、逸脱・自己判断した点、commit message の草案。stopped / blocked の場合は理由と判断点。
- Implementer が session / turn 上限、完了通知待ち、または未完了 handoff で止まった場合は、fresh 再起動より先に同一 Implementer へ `SendMessage` して再開する。再開条件と fresh recovery への切替条件は「Subagent Progress and Recovery」を正とする。
- Implementer / Gatekeeper / subagent の報告どうしが食い違う場合、Conductor はどれかを採用する前に実ソース・実測で裏取りしてから記録・報告する。裏取りで決着しない場合だけ Advisor に相談する（Advisor 節）。
- 直接実行の例外は Goal 経由の作業には適用しない。Goal を経由しない単発 Change だけは、現在の agent が直接実行してよい。

## Gatekeeper

- Gatekeeper は Change ごとに起動する fresh subagent で、実装文脈を引き継がない。実体は Goal 指定のモデルで決まる（無指定は `models.md` の役割既定。指定できるのは Claude 系短名のみ。モデル階級を上げてレビュー検出力が上がった実測がないため、既定を上げない。Implementer の High-risk 引き上げとは非対称だが、意図的な非対称）。
- 適用範囲: Normal 以上の全 Change に入れる（暫定。planted-defect 実験の結果次第で High-risk 限定へ縮退する可能性がある）。Small は省略し、Conductor が diff を直接実読して照合する（縮退）。この縮退は「Conductor は per-commit 詳細を読まない」不変条件の明示的な例外である。Small の diff は小さく、Conductor が読む複利負荷が無視できることを根拠にした限定的な例外に限る。読み始めて Small の想定を超える差分量・複雑さだと分かった場合は、その場で Gatekeeper 起動に切り替える（Small 判定のまま Conductor が読み続けない）。
- 責務: diff 全量の実読、Change brief（scope・変更対象領域・Acceptance・非対象・設計制約。Conductor が Implementer 起動時に確定し、Gatekeeper にも Conductor が確定した最新版を渡す）・plan との照合、テストの再実行による裏取り、review lane（`change/review.md` の Standard / Targeted supplement / External supplement）の起動と統合、指摘の採否、受け入れ判定。
- Implementer の報告は入力にしない。diff・テストという実物を直接見る。Implementer とは会話せず、差し戻しは Conductor 経由で行う（Conductor が同一 Implementer を `SendMessage` で再開させる）。
- **差し戻し上限（2 往復）**: 上限に達しても未解決の MUST が残る場合は commit せず停止してユーザーに確認する（Stop Conditions）。残りが SHOULD 以下のみの場合に限り、Gatekeeper が残リスク受容を明示して accept したときだけ commit に進める。「残リスクを記録すれば自動的に commit に進める」運用はしない。
- **brief 外ファイルの照合**: commit に含める予定の全ファイル（tracked の変更・untracked の新規追加を含む）のうち、Change brief の変更対象領域に含まれないものがあれば、内容の正誤に関わらず scope 超過として差し戻す（通常の差し戻しと同じく上限 2 往復にカウントする）。判定は「ファイルが brief の変更対象領域に含まれるか」の機械的な照合であり、修正の必要性の意味判断はしない。正当な波及だった場合は、Conductor が brief の変更対象領域を更新してから同一 Implementer に再提出させる。brief 更新は Final Report に記録する。上限に達しても scope 超過が未解消（brief 外ファイルが残る）の場合は、未解決の MUST と同等に扱い、SHOULD 以下の残リスク受容経路には流さない。
- 実装対象がない修正（docs のみ、typo 級等）であっても Gatekeeper は直接編集しない。すべての修正は Conductor 経由で Implementer に差し戻す（複数の指摘は 1 回の差し戻しにまとめる）。自分の修正を自分で受け入れると採否の独立性が崩れ、下記の diff hash 照合の前提（Gatekeeper 通過後に diff が変異しないこと）も壊れるため。
- 戻り値には実行証拠を必須とする: baseline HEAD SHA、`git status --porcelain` の対象状態、commit 予定差分全体のハッシュ（算出方法は `change/finish.md` の機械照合を参照）、実行したテストコマンド・exit code・所要時間（成功時はこれのみ。生の出力 tail は失敗時に限り返す）、読んだ diff の stat、commit に含める予定の全ファイル（tracked の変更・untracked の新規追加を含む）のうち Change brief の変更対象領域に含まれないものの一覧（なければ「なし」）、起動した review lane の記録、判定（accept / 差し戻し）、採用した指摘と対応要求、後続 Change への影響事実、残リスク、Product Decision Ledger 候補。「確認済み」という宣言だけの報告は受理しない。
- 存在理由は検出力の高さそのものではなく、(i) 指摘採否を実装文脈から独立させること、(ii) 裏取り手続き（テスト再実行・diff 実読）の構造的な実行点を作ること、(iii) per-commit の詳細を Conductor の複利コンテキストから隔離すること、の 3 点にある。中身のバグ検出の主役は従来どおり review lane の finder subagent と Goal Review であり、Gatekeeper 自身の役割は独立した実行点の確保にある。
- Goal Review が MUST を出した場合、その欠陥がどの Gatekeeper 手続きをすり抜けたかを Final Report に記録する（すり抜け記録義務、分類は goal.md 冒頭の Constraints 参照）。

## Advisor

判定が割れて Conductor の機械的裏取りでは決着しないときに、方向を決めるためだけに呼ぶ役割。常設の工程ではない。

- 実体は読み取り専用の fresh subagent（モデルは `models.md` の役割既定。Claude 側は `fable` 固定）。Conductor が起動し、割れている論点・両者の主張・Conductor が実施した裏取りの結果・関連する実ファイルのパスを渡す。
- **発火条件はこの 2 つだけ**。これ以外では呼ばない:
  - (a) Implementer と Gatekeeper の主張が割れ、Conductor の機械的裏取り（`git show`、実ソースの Read）で決着しない。
  - (b) Goal Review と per-Change Gatekeeper の判定が割れ、Conductor の機械的裏取り（該当 commit の `git show`、実ソースの Read）で決着しない。
- 返すのは方向の判定と根拠だけ。ファイルを書かない・修正案を実装しない（実装者にしない）。tracked file の編集、git 書き込み、テスト以外の副作用のあるコマンドを禁じる。
- Advisor の相談は差し戻しの往復にカウントしない（Gatekeeper の差し戻し上限 2 往復、Goal Review の reviewer 呼び出し上限 3 回のどちらにも数えない）。
- Advisor の判定は Conductor の判断材料であって、受け入れ判定の代行ではない。採否・commit の責任は従来どおり Gatekeeper と Conductor に残る。判定が Stop Conditions に該当する product decision に触れる場合は、Advisor の結論があっても停止してユーザーに確認する。

Advisor とは別役割として **Auditor** を置く。Advisor は当事者が自分の文脈を説明して助言を受ける相談席であり、Auditor は当事者の説明を遮断し証拠だけで評価する監査席で、同居させない。

- 発火条件は、差し戻し上限（2 往復）に達しても未解決の MUST が残り、Conductor が停止報告する場合だけ（Stop Conditions 参照）。Conductor は停止報告の前に Auditor を起動する。
- 実体は読み取り専用の fresh subagent（モデルは `models.md` の役割既定）。差分・ファイル・git 状態を一切変更しない（監査対象を自分で変えないため）。Implementer / Gatekeeper / Conductor の会話文脈・説明・言い分は一切渡さない。当事者の説明を聞くと判定が汚染されるため、これが Auditor の生命線。
- 渡すのは Change brief、git の生データ（HEAD、diff stat、変更ファイル一覧、diff 本文〈全文。続行/縮小/破棄の判定は diff 本文なしでは根拠づけられないため、大きい場合も stat だけで済ませない〉）、正本（backlog の該当項目、関連する ADR / specs）のみ。
- 出力は「続行 / 縮小 / 破棄して再設計」のいずれかの推奨と根拠。続行 = 現差分のまま進める（膨張は正当な波及）。縮小 = 差分の一部だけ残し、残りは別 Change へ切り直すか捨てる。破棄 = 未コミット差分を捨て、得られた知見（発見したバグ・必要な不変条件・再現テスト）だけを列挙して持ち帰り、fresh Implementer が最小設計で組み直す。
- Auditor の推奨は拘束しない。決定は常にユーザー。Conductor は Auditor の推奨を添えて停止報告する。
- Auditor を起動できない場合は、停止自体は妨げない。停止報告に「Auditor 推奨なし（起動不可）」と明示してそのまま停止報告する（Advisor の発火条件不在時と異なり、Auditor 起動不可は新たな Stop Conditions の対象にしない）。

## Subagent Progress and Recovery

この節は Implementer と Gatekeeper の両方に適用する。

- Task / Bash の timeout は Conductor 側の polling / process window にすぎず、subagent の失敗・終了・session / turn 上限を自動的には意味しない。Goal workflow は固定の経過時間だけで agent を巻き取らない。
- 結果や完了通知が返らない場合は、同一 subagent へ `SendMessage` で status request を送り、現在 phase / 実行中コマンド / 直近の実質進捗 / 残作業 / blocker を求める。既知の長時間コマンドが動いている、または応答に実質進捗がある場合は同じ subagent を継続し、fresh recovery を起動しない。コマンド状態が変わるまで同じ status request を反復しない。
- running subagent が status request にも応答しない場合だけ `TaskStop` で停止する。終了を確認するまで別 writer を起動しない。終了確認後、Conductor は `git status --short`、`git diff --stat`、必要な `git diff`、`git log --oneline -n` で実状態を確認する。
- completed / idle subagent が Acceptance 未達、session / turn 上限、時間切迫、または未完了 handoff を返した場合、待機では続行しないが、fresh agent より先に同じ subagent へ `SendMessage` する。残作業と優先順位だけを渡し、既に完了した調査・実装・検証を再実行させない。
- fresh recovery へ切り替えるのは、同じ subagent を再開できない、文脈または差分が今回の Change と整合しない、または同じ subagent の再開 2 turn で実質進捗が観測できない場合に限る。実質進捗は、scope 内の diff / artifact の変化、新しい test / build / review evidence、または根拠付きで blocker・残作業が縮小したことのいずれかで判断し、status 文面の更新だけでは進捗とみなさない。単なる timeout、自己申告の時間切迫、正常な未完了 handoff は fresh recovery の条件にしない。
- stopped / blocked は未完了 handoff と区別し、同じ入力のまま自動再開しない。Stop Conditions または不足する判断を解消してから再開する。
- recovery が必要な場合、元 subagent の終了を確認し、未コミット差分が今回の Change scope 内にあると判断できた後だけ、元が Implementer なら fresh recovery Implementer、元が Gatekeeper なら fresh recovery Gatekeeper に同じ担当作業を続行させる。commit は引き続き Conductor が行う。Conductor は実状態の確認と引き継ぎに留め、実装・判定を代行しない。差分が scope 外、破壊的、または完了状態を判断できない場合は、差分を破棄せず停止してユーザー確認する。
- この回収手順は例外処理であり、通常の Implementer / Gatekeeper に定期報告ファイルや常時 ledger を要求しない。

## Goal Review

各 commit の Gatekeeper 判定とは別に、Goal の commit range を対象に Goal Review を Goal 完了条件として実施する。ブランチは切らないので、レビュー range は commit range で表す。分割レビューの未レビュー対象は実行直前に固定する `<review_cursor>..<review_end>`、Goal 全体の差分は `<base>..HEAD`。Goal range に対して Change ごとの Standard Change Review は再実行しない。

Change Review（Gatekeeper が起動する review lane を含む）は個々の commit の局所的な correctness / spec / tests を見る。Goal Review は commit 間の統合、Goal Acceptance、構築・read / write site の貫通、docs / backlog 整合を見る。

- **実行手順の正本は `goal-review.md`**: Goal Review を実施する直前に毎回 `goal-review.md` を Read し、その内容だけに従って reviewer を起動する。この節が持つのは実施タイミングの規定だけ。
- **分割レビュー**: 一気に全部ではなく、適当なコミットのまとまりごとにレビューしてよい（毎回でなくてよい）。1 commit ごとではなく、関連する数 commit をまとめてレビューする。PASS 相当なら `review_cursor` を `review_end` まで進める（`base` は動かさない）。次のレビューは新しい `review_cursor` から、実行直前に新しい終点 SHA を取り直す。
- 差分が大きい、または永続化 / 同期 / 外部 API / 広い UI 挙動に触れる場合は、数 commit を待たずにその時点までの commit range で早めにレビューする。
- Goal 完了までに、`base` から Goal の最終 commit までの全 range がレビュー済みであること（通過条件は `goal-review.md`）。

## Final Report

- 報告を書く直前に `reporting` skill を読み込み、それに従う。
- 完了時も停止時も、報告形式は状況に合わせて分かりやすく整える。固定テンプレートに無理に合わせない。
- `ユーザー判断が必要: なし` または必要な判断内容を必ず明示する。
- `ユーザー判断が必要` は `.claude/workflow/design-decision-record.md` の基準で、各 Change の ledger、review 結果、同期済み docs から判断する。記憶だけで `なし` と判断しない。
- Goal Review の結果（各ラウンドの range・実際に呼び出した reviewer skill・verdict・指摘件数）は `tmp/workflow/<scope>/goal-review-log.md` から転記する。記憶で再構成しない。
- `レビュー上限超過: なし` または対象単位・回数・最後の指摘・行った修正・最終修正が未レビューであること・残リスクを状況に合わせて明示する。収束した review も、どの review が通ったかを状況に合わせて報告する。
- 停止時は、停止理由と解決すべきことが分かるようにする。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が `change/workflow.md` の判断境界で Stop に該当する重要な仕様・UX・データ保持・削除方針に依存している。
- Goal の途中で、現在の目的と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- 必須の検証を代替手段でも裏付けられず、完了扱いにできない。
- Goal Review の 2 本の reviewer のどちらかを完全に実施できない。
- Goal Review の 2 本の reviewer で指摘が割れ、機械的な合流では解けない。
- advisor ツールが使えない環境で、Implementer が High-risk や設計判断の厚い Change に当たった。
- Implementer ↔ Gatekeeper の差し戻しが上限 2 往復に達しても、未解決の MUST が残っている（停止報告の前に `Advisor` 節の Auditor に従い起動し、その推奨を添えて報告する）。
- Goal の呼び出し文で Implementer / Gatekeeper に GPT 系の短名が指定された（この workflow は Claude 系モデルしか起動できない）。
- Advisor の発火条件に該当したが、Advisor を起動できない。
