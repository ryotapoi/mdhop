# Change Workflow

この workflow は、単発の実装依頼、または Goal 内で切り出された 1 commit 分の作業を完了させるための手順。
Goal 経由の場合は `goal-workflow` skill を入口とし、各 commit でこの workflow を繰り返す。単発依頼の場合はこの workflow を直接の入口とする。

## Intent

単発依頼、または Goal 内で切り出された 1 commit 分の作業を、必要十分な調査・計画・実装・検証・記録で完了させる。手続きの重さは、作業の大きさとリスクに合わせる。

## Inputs

- ユーザー依頼
- 関連する `docs/rules/`, `docs/specs/`, `backlog/backlog.md`, `docs/decisions/`, `llm-wiki/`（作業地図）
- 既存コードと git history

## Intake 分類

最初に作業を分類する。判定が揺れたら High-risk 寄りに倒す。Small / Normal の境界は迷ったら Normal で進めてよい。

- **Small**: typo、文書、テスト期待値、1 ファイルの明確な修正
- **Normal**: 通常の機能追加・バグ修正・複数ファイル変更
- **High-risk**: データ永続化、マイグレーション、並行性、公開 API、削除、広い UI 挙動、外部連携
- **Exploratory**: 原因不明、仕様不明、技術検証が先に必要

## Routing

三役の分担は Goal 経由でのみ成立する。Goal 経由では Implementer → Gatekeeper（Normal 以上）→ Conductor の順に担当し、commit は常に Conductor が行う。Goal を経由しない単発 Change では Gatekeeper / Conductor という役割分担自体が存在しないため、現在の agent が実装・review 差配・採否・commit のすべてを担う。以下の Routing 各項目は、断りがない限り Goal 経由の担当を示す。

- Exploratory → `change/investigate.md` で事実を揃えてから判断し直す
- 実装 → Implementer のモデル指定で決まる（無指定 `sonnet`）。Claude 系 = Implementer 自身が計画と実装を一体で行う。GPT 系 = Implementer が codex（外部実装エージェント）となり、Claude 側は watchdog subagent が運転だけを行う（`change/delegate.md`）。モデル指定は Implementer の実体だけを決め、下記の各 phase・Gatekeeper・Conductor・commit の進行は指定によらず共通
- Plan が必要な変更 → `change/plan.md`（plan mode は使わず、内部で計画を立ててそのまま `change/implement.md` へ進む。詳細は `change/plan.md`）
- Plan 省略可な変更 → そのまま `change/implement.md`
- 検証 → `change/verify.md`
- レビュー → `change/review.md`（Normal 以上は Gatekeeper が起動する。Small は Conductor が diff を直接実読して照合する。単発 Change では現在の agent が直接照合する）
- 完了 → `change/finish.md`（commit は Conductor が行う。単発 Change では現在の agent が行う）
- 節目で構造を見る → `maintenance.md`

## Decision Criteria

- workflow は 1 つの commit 単位で回す。1 commit に独立した複数作業を混ぜない。
- 実行中に 1 commit として不自然だと分かったら、作業を広げず、Goal 実行中は Conductor に事実を返して commit 単位を切り直す。単発 Change では今回扱う単位を切り直す。
- Small は plan を省略してよい。作業内容と検証だけ簡潔に示す。
- 仕様・UX・データモデル・複数ファイル変更・設計判断を伴うなら plan を作る。
- High-risk は plan・検証・必要なレビューを明示する。
- 実装判断に影響する不明点は、調査・検証・既存情報で潰してから進む。仕様・UX の不明点は、現在の要求、正本、既存コード、調査・検証結果から採用案を選んで進める。可逆で影響が小さい選択は、Product Decision Ledger の対象なら ledger に残す。複数の妥当案が残り、かつ選択が非可逆（データ保持・削除・マイグレーション・外部公開契約）またはやり直しコストが大きい場合、または正本と矛盾する場合は Stop Conditions に従う。
- Product Decision Ledger の対象・Alternative Check・報告基準（UX・データ意味・cross-surface 等。カテゴリ一覧は同ファイル）は `.claude/workflow/design-decision-record.md` を唯一の正本とする。
- 途中でタスクの性質が変わったら、Intake からやり直す（格上げは許容）。
- 既存 worktree 差分向けの特別な snapshot / staging / clean check フローは作らない。通常の差分確認と commit discipline で巻き込みを防ぐ。

## Source Resolution

現在のユーザー依頼は作業の目的を定める。`docs/rules/`、`docs/decisions/`、`docs/specs/`、tests は正本と根拠として照合する。矛盾した場合は依頼を理由に正本を黙って上書きせず、Stop Conditions に従ってどの情報源が古いかを確定してから同期する。

Product Decision Ledger は新しい正本ではない。Goal、長い Change、委任、review 指摘対応をまたぐ判断候補がある場合は、必要に応じて `tmp/product-decision-ledger/<scope>.md` に残す。finish では記憶ではなく ledger、review 結果、同期済み docs から `ユーザー判断が必要` を判断する。

## Phase Handoff

固定テンプレートは要求しないが、phase を移る時は次に必要な事実を欠落させない。Goal 経由では Implementer → Gatekeeper（Normal 以上）→ Conductor の順に引き継ぐ（Small は Implementer → Conductor）。

- 扱った scope と result（Implementer が起点）
- 検証コマンド・結果、review status（Implementer が実行し、Gatekeeper が裏取り・統合）
- commit SHA、または未 commit / stop の理由（commit は Conductor が確定する）
- docs / backlog 同期、Product Decision Ledger、follow-up、残存リスクの有無

## Acceptance

- ユーザーの要求が満たされている
- 必要な情報源が同期されている（`docs/specs/`, `backlog/backlog.md`, `docs/decisions/`、知見はソースコメント / `llm-wiki/`）
- 選んだ検証とレビューの深さを説明できる
- コミット済み、またはユーザーが明示的にコミット不要とした状態
- コミット後の進み方は `change/finish.md` に従う（Goal 実行中は次の 1 commit workflow へ、Goal 外の単発依頼はユーザー指示待ち）

## Stop Conditions

- Decision Criteria の判断境界で Stop に該当する仕様・UX・データ保持・削除方針が残っている
- 要求と `docs/rules/` / `docs/specs/` / `docs/decisions/` が矛盾している
- High-risk 変更で必須の検証を代替手段でも裏付けられない
- ユーザーが停止・相談・計画のみを指示している

## Subagent / Skill

- 複数ファイル横断・キーワードのファンアウト調査は Explore subagent に委譲する
- 互いに独立した read-only 調査・レビューは並列化してよい。同一 worktree の実装 writer は 1 つに限る
- subagent の完了通知は配信されない・大幅に遅延することがある。background 起動して完了通知を待つ形を避け、結果は起動呼び出しの戻り値で受け取る。background になった・結果が返らない場合は通知を待たず `SendMessage` で能動的に結果を請求する。通知待ちの待機ループ（no-op の Monitor / sleep の積み増し）は行わない
- skill は各 phase の workflow の指示に従って使う。
- 横断のスコープ判定は `boundary-control` を正本とし、全 phase に効かせる。今回の要求の外へ作業を広げそうな時、隣接作業が見つかった時、scope を変える編集の前に使い、active scope 内か（workflow-required / incidental-required）を判定する。隣接作業は現在の commit に広げず、project-relevant なら workflow が認める正本へ capture するか最終報告で report する。active workflow を止めたり置き換えたりはしない
- 詳細は各 phase のファイル参照
- Product Decision Ledger の判断基準は `.claude/workflow/design-decision-record.md` を参照する
