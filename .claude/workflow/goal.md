# Goal Workflow

この workflow はこのプロジェクトの Goal 手順の正本。実装作業の発火入口は `goal-workflow` skill とし、`goal-workflow` skill はこのファイルを読んで進める。

> 役割名の改名について: この canon は役割を 2 層（Orchestrator/Implementer）から 3 層（Conductor/Implementer/Gatekeeper）に変更した。旧 Orchestrator は Conductor に改名し、責務も「受け入れ判定」から「進行管理・機械照合・commit 実行」に変わっている（diff の実読と受け入れ判定は新設の Gatekeeper が担う）。canon 外（skills、agent 定義等）に Orchestrator 表記が残っている場合は、意味上は Conductor または Gatekeeper のどちらかに読み替える。

## Intent

`/goal` で指定された目的を、複数の 1 commit workflow に分割して完了まで進める。

## Constraints

- 実装作業は `goal-workflow` skill を入口にし、この workflow を正本として読む。
- `/goal` の呼び出し文は、原則として skill への参照と完了対象だけでよい。例: `/goal goal-workflow skill に従い、backlog/backlog.md の「v0.x」を完了して。`
- 役割は 3 層で固定する:
  - **Conductor**（この workflow を進める main セッション。commit slicing・次 Change 選定・Change brief 確定・subagent 起動・機械照合・commit 実行・Goal Review 手配・停止判断・最終報告を担う。実装は書かず、per-commit の詳細（diff 本文、テストログ、review 往復）も読まない。受け取るのは Implementer / Gatekeeper からの構造化要約だけで、実物の diff を読むのは報告に食い違い・疑義がある例外時に限る。Small での直接 diff 照合は、この不変条件の明示的な例外である。詳細は Gatekeeper 節参照）。
  - **Implementer**（Change ごとに fresh subagent。計画・実装・検証を担う。commit はしない。review lane は回さない）。
  - **Gatekeeper**（Change ごとに fresh subagent、実装文脈を引き継がない。diff 全量の実読、Change brief・plan との照合、テスト再実行による裏取り、review lane の起動・統合、指摘の採否、受け入れ判定を担う。Implementer の報告は入力にせず、diff・テストという実物を直接見る。Gatekeeper の詳細は Gatekeeper 節を参照）。
- `/goal` の呼び出し文で Implementer / Gatekeeper のモデルを役割ごとに指定できる（例: `implementer: terra, gatekeeper: sol`）。短名の後ろに reasoning effort を添えて役割ごとに明示してもよい（例: `implementer: luna xhigh`）。無指定は両役割とも既定（短名・序列・effort の有効値と既定は `models.md` を正とする）。transport はモデルの系統から自動で決まる: Claude 系はネイティブ subagent、GPT 系は codex exec + watchdog（`change/delegate.md`）。Conductor・commit・Goal Review はどの組み合わせでも完全共通。Claude 系 Implementer は計画と実装を一体で行い、実装前の plan 書き出しは `change/plan.md` に従う。旧語彙の読み替え: `solo` = 両役割とも Claude 系既定、`delegate` = Implementer に GPT 系を指定した状態。使い分けの目安: 重要部分（High-risk 相当が中心の Goal）は既定のまま、それ以外で Claude 側の使用量を抑えたい Goal は GPT 系を座らせる（さらに抑える場合は Gatekeeper にも GPT 系）。指定は原則 Goal 全体で固定し、Change 単位で黙って差し替えない（High-risk での引き上げは Implementer 節の条項に従う）。委譲が実行不能な場合の扱いは `change/delegate.md` の Stop Conditions に従う。
- ブランチは切らず、いるブランチ（通常 main）上にそのまま 1 commit ずつ積む。Goal 開始時の `HEAD` を base SHA として記録する（Goal Review の range 起点）。
- 1 回の実装 workflow は 1 commit 単位に限る。Conductor は実装を直接担当せず、Goal が 1 commit だけで完了する場合も、次の 1 Change を選んで fresh subagent を Implementer として 1 つずつ直列起動する。Implementer の完了後は Gatekeeper（Normal 以上）を起動し、通過したら Conductor が機械照合のうえ commit する。モデル指定に関わらずこのフローは共通（GPT 系では Implementer の transport が変わるだけ）。
- 各 commit は、Goal 全体の途中でも、その commit 単位では review / revert / bisect できる完了状態にする。
- Goal 全体を 1 plan / 1 commit に押し込まない。次に扱う 1 commit 分を毎回明確に切り出す。
- Goal 前提では都度のユーザー確認を避け、自動進行する。止まるのは Stop Conditions に該当する場合だけ。
- plan mode（`EnterPlanMode` / `ExitPlanMode`）は使わない。承認待ちが自動進行と噛み合わないため。計画が必要な場合は内部で立ててそのまま実装する。詳細は `change/plan.md`。
- 複数案の判断は `change/workflow.md` の境界に従う。可逆で影響が小さい選択は採用案で進め、複数の妥当案が残り、かつ選択が非可逆またはやり直しコストが大きい場合、または正本と矛盾する場合は Stop Conditions に従う。
- 仕様・UX の不明点は `change/workflow.md` の判断境界に従う。Product Decision Ledger の対象・記録・報告基準は `.claude/workflow/design-decision-record.md` を正本とし、Goal 完了時は各 Change の ledger、review 結果、同期済み docs から `ユーザー判断が必要` の有無を集約する。
- 進捗・完了の報告は、このセッションのツール結果で裏取りできる事実だけを書く。テストが失敗していれば出力ごと報告し、未検証の項目は未検証と明示する。
- 後から制約になる判断、仕様変更、未着手作業は、画面出力だけで終わらせず `docs/rules/` / `docs/specs/` / `docs/decisions/` / `backlog/backlog.md` の適切な情報源へ同期する。
- 各 commit の Gatekeeper 判定とは別に、Goal の commit range に対する Goal Review を Goal 完了条件に含める（Goal Review 参照）。reviewer は実装文脈を引き継がない fresh reviewer とする。fresh であることを必須とし、実装と同系統でも fresh なら reviewer になれる（reviewer の選定は Goal Review 参照）。Goal range に `/code-review` 観点ベースの再レビューは再実行しない。
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
- `.claude/workflow/change/delegate.md`（codex transport: GPT 系モデルを役割に指定したときの運転手順）
- `.claude/workflow/models.md`（モデルの短名・序列・reasoning effort の正本）
- `.claude/workflow/change/review.md`（Gatekeeper が起動する review lane）
- `.claude/workflow/design-decision-record.md`
- `design-decision` skill
- `codex-fresh-review` skill（全 mode の Goal Review で使う）
- `claude-fresh-review` skill（Goal Review では既定で使わない。ユーザーが Goal 指定で明示した場合のみ `codex-fresh-review` に追加する）
- `backlog/backlog.md`

## Flow

1. Goal の目的、制約、完了条件、役割ごとのモデル指定（無指定は `models.md` の役割既定）を確認し、ブランチは切らず開始時の `HEAD` を base SHA として記録する。
2. Goal を 1 commit 単位の候補へ分割する（Commit Slicing 参照）。
3. 次に扱う 1 commit 分を選び、Change brief（scope・Acceptance・非対象・設計制約）を確定して fresh Implementer に渡す（Goal が 1 commit だけの場合も同じ）。モデル指定は Implementer の実体を決めるだけで、起動手順そのものは共通。同じ Change brief は Gatekeeper 起動時にも不変入力として渡す（Implementer の報告経由で伝えない）。
4. Implementer の完了後、Intake が Small なら Conductor が diff を直接実読して照合する（Gatekeeper 省略）。Normal 以上なら fresh Gatekeeper を起動し、diff 実読・テスト再実行・review lane・受け入れ判定を行わせる。差し戻しがあれば Conductor 経由で同一 Implementer を `SendMessage` で再開させる（上限 2 往復。上限超過時の扱いは差し戻し上限の節を参照）。
5. Gatekeeper が通過（または Small で Conductor が直接照合）したら、Conductor が機械照合（Gatekeeper 報告の baseline HEAD SHA の実在確認、意図しない git 書き込みの有無、diff --stat の一致、commit 予定差分全体のハッシュの再計算・一致確認、テストの自己実行〈exit code のみ確認〉）を行う。テスト自己実行が worktree を変更し得るため、実行後に `git status` と diff hash を再照合してから commit する。
6. commit 後、Goal の残りと Goal Review の実施タイミングを確認する。残りがあれば手順 3 に戻る。
7. 必要な Goal Review と対応が済んでいなければ実施する。実行直前の `HEAD` を `review_end` として固定し、PASS 相当なら `review_cursor` を `review_end` まで進めてよい（`base` は動かさない）。Goal Review が MUST を出した場合は、すり抜け記録義務に従い該当欠陥がどの Gatekeeper 手続きをすり抜けたかを記録する。
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

- Implementer の実体は Goal 指定のモデルで決まる（無指定は `models.md` の役割既定）。Claude 系は Implementer（Claude subagent）自身が計画・実装する。GPT 系は Implementer が codex（外部実装エージェント）となり、調査・実装計画・実装・検証・設計質問の内部解決を一体で担い、Claude 側は watchdog subagent が運転だけを行う（`change/delegate.md`）。どちらも Implementer は commit せず、review lane も回さない。
- Goal 経由の Change は、commit 数に関わらず、原則 fresh Implementer に渡す。
- Conductor は実装を直接担当しない。Conductor の責務は、base / review_cursor 管理、commit slicing、次の Change 選定、subagent 起動、機械照合、commit 実行、Goal Review 手配、最終報告に限る。
- Conductor は次の 1 Change を選び、Change brief（scope・Acceptance・非対象・設計制約）を確定した上で fresh subagent を Implementer として 1 つずつ直列起動する。同じ worktree で複数の Implementer を並行実行しない。Goal が 1 commit だけで完了する場合も Implementer を 1 つ起動する。
- Implementer の起動も、結果を起動呼び出しの戻り値で受け取る同期実行を基本とする。background になった場合は完了通知を待たず、`SendMessage` で能動的に結果を回収する（`change/workflow.md` の Subagent / Skill 参照）。
- モデルと effort はベンダー推奨既定で運用し、effort は動かさない（既定は `models.md` を正とする。watchdog は `sonnet` 固定）。ユーザーの明示指定（モデル・effort とも）が常に最優先で、呼び出し文で effort が明示された役割はその値で起動する。High-risk Change で、文脈を十分与えても誤る「問題が難しい」型の失敗が観測された場合に限り、Conductor は同系統の 1 段上のモデル（`models.md` の序列。最上位の場合は引き上げ先なし）への引き上げを検討してよい（既定は引き上げなし。実施したら最終報告に理由と結果を記録する。系統は跨がない）。読み飛ばし・検証不足など「頑張りが足りない」型の失敗は、引き上げでなく契約項目（全列挙・検証義務）と差し戻しで直す。この判断軸は Anthropic の公式ガイダンス（既定 effort で明確に試みても誤るならモデルのサイン）由来で、GPT 系への適用は未検証の作業仮説。それ以外では難度を理由に引き上げない。この引き上げは失敗観測後の是正であり、advisor（Claude 系の予防的相談、下記）とは別枠で併用する。advisor 不在の代替として引き上げを使わない。
- advisor が設定されている環境（Claude Code の `advisorModel` 等。subagent は設定を継承する）では、Claude 系 Implementer の起動プロンプトに advisor の相談条件を含める（advisor は Claude 実装の精度向上手段であり、watchdog には含めない）: 非自明な設計判断にコミットする前、同じエラー・失敗が繰り返す時、アプローチの変更を検討する時に advisor に相談する。自明な作業では呼ばない。
- High-risk や設計判断の厚い Change を Claude 系 Implementer で進める場合は、advisor 相談を厚くする: 相談条件に加えて、実装方針の確定前と完了宣言前の相談を必須と明記する。advisor が使えない環境で Claude 系 Implementer が High-risk Change に当たった場合は、モデルを引き上げて代替せず停止してユーザーに確認する（Stop Conditions 参照）。
- Implementer は渡された Change だけを担当し、Goal 全体を再計画・再分割しない。
- 通常は `change/workflow.md` に従い、調査から実装・検証まで完了して戻る（commit と review lane の起動は含まない）。
- 1 commit として不自然だと分かった場合は、作業を広げず事実を Conductor に返す。Conductor が commit 単位を切り直す。
- 戻りの表示形式は固定しないが、次を必ず引き継ぐ: 終了種別（completed / stopped / blocked / interrupted のいずれか）、plan 参照、変更ファイル一覧、実行した検証コマンドと結果、逸脱・自己判断した点、commit message の草案。stopped / blocked の場合は理由と判断点。
- Implementer が session / turn 上限、完了通知待ち、または未完了 handoff で止まった場合は、fresh 再起動より先に同一 Implementer へ `SendMessage`（GPT 系委譲は同一 codex session の resume）して再開する。再開条件と fresh recovery への切替条件は「Subagent Progress and Recovery」を正とする。
- Implementer / Gatekeeper / subagent の報告どうしが食い違う場合、Conductor はどれかを採用する前に実ソース・実測で裏取りしてから記録・報告する。
- 直接実行の例外は Goal 経由の作業には適用しない。Goal を経由しない単発 Change だけは、現在の agent が直接実行してよい。

## Gatekeeper

- Gatekeeper は Change ごとに起動する fresh subagent で、実装文脈を引き継がない。実体は Goal 指定のモデルで決まる（無指定は `models.md` の役割既定。モデル階級を上げてレビュー検出力が上がった実測がないため、既定を上げない。Implementer の High-risk 引き上げとは非対称だが、意図的な非対称）。GPT 系を指定した場合は、実装セッションとは別の fresh codex セッションを Gatekeeper とし、起動・運転・review lane の代替は `change/delegate.md` の Gatekeeper 委譲に従う。実体がどちらでも、本節の責務・戻り値の実行証拠・差し戻し運用は変わらない。
- 適用範囲: Normal 以上の全 Change に入れる（暫定。planted-defect 実験の結果次第で High-risk 限定へ縮退する可能性がある）。Small は省略し、Conductor が diff を直接実読して照合する（縮退）。この縮退は「Conductor は per-commit 詳細を読まない」不変条件の明示的な例外である。Small の diff は小さく、Conductor が読む複利負荷が無視できることを根拠にした限定的な例外に限る。読み始めて Small の想定を超える差分量・複雑さだと分かった場合は、その場で Gatekeeper 起動に切り替える（Small 判定のまま Conductor が読み続けない）。
- 責務: diff 全量の実読、Change brief（scope・Acceptance・非対象・設計制約。Conductor が Implementer 起動時に確定し、Gatekeeper にも不変入力として渡す）・plan との照合、テストの再実行による裏取り、review lane（`change/review.md` の Standard / Targeted supplement / External supplement）の起動と統合、指摘の採否、受け入れ判定。
- Implementer の報告は入力にしない。diff・テストという実物を直接見る。Implementer とは会話せず、差し戻しは Conductor 経由で行う（Conductor が同一 Implementer を `SendMessage` で再開させる）。
- **差し戻し上限（2 往復）**: 上限に達しても未解決の MUST が残る場合は commit せず停止してユーザーに確認する（Stop Conditions）。残りが SHOULD 以下のみの場合に限り、Gatekeeper が残リスク受容を明示して accept したときだけ commit に進める。「残リスクを記録すれば自動的に commit に進める」運用はしない。
- 実装対象がない修正（docs のみ、typo 級等）であっても Gatekeeper は直接編集しない。すべての修正は Conductor 経由で Implementer に差し戻す（複数の指摘は 1 回の差し戻しにまとめる）。自分の修正を自分で受け入れると採否の独立性が崩れ、下記の diff hash 照合の前提（Gatekeeper 通過後に diff が変異しないこと）も壊れるため。
- 戻り値には実行証拠を必須とする: baseline HEAD SHA、`git status --porcelain` の対象状態、commit 予定差分全体のハッシュ（算出方法は `change/finish.md` の機械照合を参照）、実行したテストコマンド・exit code・所要時間（成功時はこれのみ。生の出力 tail は失敗時に限り返す）、読んだ diff の stat、起動した review lane の記録、判定（accept / 差し戻し）、採用した指摘と対応要求、後続 Change への影響事実、残リスク、Product Decision Ledger 候補。「確認済み」という宣言だけの報告は受理しない。
- 存在理由は検出力の高さそのものではなく、(i) 指摘採否を実装文脈から独立させること、(ii) 裏取り手続き（テスト再実行・diff 実読）の構造的な実行点を作ること、(iii) per-commit の詳細を Conductor の複利コンテキストから隔離すること、の 3 点にある。中身のバグ検出の主役は従来どおり review lane の finder subagent と Goal Review であり、Gatekeeper 自身の役割は独立した実行点の確保にある。
- Goal Review が MUST を出した場合、その欠陥がどの Gatekeeper 手続きをすり抜けたかを Final Report に記録する（すり抜け記録義務、分類は goal.md 冒頭の Constraints 参照）。

## Subagent Progress and Recovery

この節は Implementer と Gatekeeper の両方に適用する。

- Task / Bash / delegate の timeout は Conductor または watchdog 側の polling / process window にすぎず、subagent の失敗・終了・session / turn 上限を自動的には意味しない。Goal workflow は固定の経過時間だけで agent を巻き取らない。
- 結果や完了通知が返らない場合は、同一 subagent へ `SendMessage` で status request を送り、現在 phase / 実行中コマンド / 直近の実質進捗 / 残作業 / blocker を求める。既知の長時間コマンドが動いている、または応答に実質進捗がある場合は同じ subagent / codex session を継続し、fresh recovery を起動しない。コマンド状態が変わるまで同じ status request を反復しない。
- running subagent が status request にも応答しない場合だけ `TaskStop` で停止する。終了を確認するまで別 writer を起動しない。終了確認後、Conductor は `git status --short`、`git diff --stat`、必要な `git diff`、`git log --oneline -n` で実状態を確認する。
- completed / idle subagent が Acceptance 未達、session / turn 上限、時間切迫、または未完了 handoff を返した場合、待機では続行しないが、fresh agent より先に同じ subagent へ `SendMessage` する。GPT 系委譲は同じ codex session を resume する。残作業と優先順位だけを渡し、既に完了した調査・実装・検証を再実行させない。
- fresh recovery へ切り替えるのは、同じ subagent / codex session を再開できない、文脈または差分が今回の Change と整合しない、または同じ session の再開 2 turn で実質進捗が観測できない場合に限る。実質進捗は、scope 内の diff / artifact の変化、新しい test / build / review evidence、または根拠付きで blocker・残作業が縮小したことのいずれかで判断し、status 文面の更新だけでは進捗とみなさない。単なる timeout、自己申告の時間切迫、正常な未完了 handoff は fresh recovery の条件にしない。
- stopped / blocked は未完了 handoff と区別し、同じ入力のまま自動再開しない。Stop Conditions または不足する判断を解消してから再開する。
- recovery が必要な場合、元 subagent の終了を確認し、未コミット差分が今回の Change scope 内にあると判断できた後だけ、元が Implementer なら fresh recovery Implementer、元が Gatekeeper なら fresh recovery Gatekeeper に同じ担当作業を続行させる。commit は引き続き Conductor が行う。Conductor は実状態の確認と引き継ぎに留め、実装・判定を代行しない。差分が scope 外、破壊的、または完了状態を判断できない場合は、差分を破棄せず停止してユーザー確認する。
- この回収手順は例外処理であり、通常の Implementer / Gatekeeper に定期報告ファイルや常時 ledger を要求しない。

## Goal Review

各 commit の Gatekeeper 判定とは別に、Goal の commit range を対象に Goal Review を Goal 完了条件として実施する。ブランチは切らないので、レビュー range は commit range で表す。分割レビューの未レビュー対象は実行直前に固定する `<review_cursor>..<review_end>`、Goal 全体の差分は `<base>..HEAD`。Goal range に対して `/code-review` 観点ベースの再レビューは再実行しない。

Change Review（Gatekeeper が起動する review lane を含む）は個々の commit の局所的な correctness / spec / tests を見る。Goal Review は commit 間の統合、Goal Acceptance、構築・read / write site の貫通、docs / backlog 整合を見る。

- **reviewer の選定（必須）**: reviewer は実装文脈を引き継がない fresh reviewer とする。fresh であることを必須とし、実装と同系統でも fresh なら reviewer になれる。Conductor 自身は Goal Review を行わない（自分が指示・監督した実装の盲点を引き継ぐため）。Gatekeeper も Goal Review の reviewer にはしない（個々の Change の受け入れ判定者であり、commit 間の統合を見る立場と分けるため）。
  - 全モデル指定共通: `codex-fresh-review` skill（実装文脈を引き継がない fresh な Codex）に依頼し、その PASS 相当を Goal Review 通過とする。実装が GPT 系の場合は reviewer が実装と同系統になるが、fresh な別インスタンスであれば可。
  - ユーザーが Goal 指定で明示した場合のみ、`claude-fresh-review` skill（fresh Claude subagent、セッション文脈非継承）を追加し、その場合は両方の PASS 相当を通過条件とする。既定では追加しない。
- **レビュー依頼に含める観点（必須）**: レビュー依頼には「変更したフィールド・型・メッセージについて、全構築サイト・全 read / write サイトを列挙して貫通漏れがないか確認する」観点を含める（複数ある組み立て経路の一部だけ修正される欠陥クラスに直効するため）。
- **Goal Review の実行（必須）**: 選定した reviewer skill を未レビュー range 対象で実行する。実行直前に `review_start = review_cursor`、`review_end = 現在の HEAD の実 SHA` を確定し、1 回の review 中は `<review_start>..<review_end>` を動かさない。
- **分割レビュー**: 一気に全部ではなく、適当なコミットのまとまりごとにレビューしてよい（毎回でなくてよい）。PASS 相当なら `review_cursor` を `review_end` まで進める（`base` は動かさない）。次のレビューは新しい `review_cursor` から、実行直前に新しい終点 SHA を取り直す。
- 1 commit ごとではなく、関連する数 commit をまとめてレビューする。
- 差分が大きい、または永続化 / 同期 / 外部 API / 広い UI 挙動に触れる場合は、数 commit を待たずにその時点までの commit range で早めにレビューする。
- 指摘対応は別 commit として作成し、対応 commit を含む range で再レビューする。follow-up review でも実行直前に新しい `review_end` を取り直す。
- 各レビュー単位につき reviewer を呼ぶ回数は、初回を含めて合計最大 3 回。`Review 1 -> Fix 1 -> Review 2 -> Fix 2 -> Review 3 -> Fix 3` まで行ったら Review 4 は行わない。Review 3 後の Fix 3 は未レビューの最終修正になるため、同じ review 単位を上限到達として打ち切り、Goal 作業は続ける。reviewer を追加している場合、回数上限と PASS / 指摘 / 上限到達は reviewer ごとに数え、最終報告で個別に分かるようにする。

## Final Report

- 完了時も停止時も、報告形式は状況に合わせて分かりやすく整える。固定テンプレートに無理に合わせない。
- `ユーザー判断が必要: なし` または必要な判断内容を必ず明示する。
- `ユーザー判断が必要` は `.claude/workflow/design-decision-record.md` の基準で、各 Change の ledger、review 結果、同期済み docs から判断する。記憶だけで `なし` と判断しない。
- `レビュー上限超過: なし` または対象単位・回数・最後の指摘・行った修正・最終修正が未レビューであること・残リスクを状況に合わせて明示する。収束した review も、どの review が通ったかを状況に合わせて報告する。
- 停止時は、停止理由と解決すべきことが分かるようにする。

## Stop Conditions

- Goal の完了条件が曖昧で、1 commit 単位へ切れない。
- 次の commit が `change/workflow.md` の判断境界で Stop に該当する重要な仕様・UX・データ保持・削除方針に依存している。
- Goal の途中で、現在の目的と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している。
- 必須の検証を代替手段でも裏付けられず、完了扱いにできない。
- Goal Review を完全に実施できない。
- advisor が使えない環境で、Claude 系 Implementer が High-risk や設計判断の厚い Change に当たった。
- Implementer ↔ Gatekeeper の差し戻しが上限 2 往復に達しても、未解決の MUST が残っている。
