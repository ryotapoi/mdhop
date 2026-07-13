# Delegate

## Intent

codex transport の共通手順。Implementer / Gatekeeper に GPT 系モデルが指定された Change は、この手順で codex に委譲する。委譲される codex セッション（外部実装エージェント）が当該役割（Implementer / Gatekeeper）の実体であり、調査・実装計画・実装・検証・設計質問の内部解決（Implementer）または diff 実読・照合・受け入れ判定（Gatekeeper）を一体で担う。内部で subagent を使うかは codex 側の裁量とし、こちらから分業を指示しない。

Claude 側は役割ごとに極薄の運転 subagent（watchdog）を 1 つ起動する。watchdog は workflow の役割ではなく運転装置であり、内容判断を一切しない（Watchdog 節参照）。委譲しても、受け入れ判定は Gatekeeper（Small では Conductor）に、commit と機械照合は Conductor に残る。

## Use When

- Goal 経由の Change で、Implementer に GPT 系モデルが指定されている（Implementer 委譲）
- Goal 指定で Gatekeeper に GPT 系モデルが指定されている（Gatekeeper 委譲。Implementer の指定とは独立）
- 使い分けの目安は `goal.md` のモデル指定の項に従う（重要部分は既定のまま、それ以外で Claude 側の使用量を抑えたい Goal は GPT 系を座らせる）
- High-risk Change での引き上げは `goal.md` の Implementer 節の条項に従う（同系統の上位モデルに上げる。effort は動かさない。系統は跨がない）。委譲のまま進められない場合は停止してユーザーに確認する

実装対象がない Change（docs / backlog 整理のみ等）は委譲の対象外とし、Conductor が直接編集してよい。これはこの workflow からの逸脱ではない。

## 呼び出し方（共通 transport）

既定の委譲先は Codex CLI。役割に指定された GPT 系短名→フル ID・effort の解決は `models.md` の表を正とし、フル ID を `-m` に渡す。reasoning effort の既定・有効値は `models.md` に従う。Goal 呼び出し文で effort が明示された役割だけ `-c model_reasoning_effort=<値>` を付け、それ以外では動かさない（High-risk 引き上げはモデル側で行う。`goal.md` の Implementer 節参照）。呼び出し方は全プロジェクト共通（すべて watchdog subagent 内で実行する）:

- 起動は `codex exec -m <モデル> -s workspace-write "<prompt>" </dev/null` を Bash で実行する（`timeout: 600000`）。調査だけなら `-s read-only` または `codex` skill を使う。
- Bash 呼び出しは `dangerouslyDisableSandbox: true` で実行する。codex 自身が OS sandbox を張るため、Claude Code の sandbox と二重になると起動に失敗する。
- 起動直後にログヘッダの `session id:` を記録する。resume は `codex exec resume <session_id> -m <モデル> -c sandbox_mode=workspace-write "<prompt>"` と session ID・モデルを毎回明示する。effort を付けた役割は resume でも `-c model_reasoning_effort=<値>` を毎回付ける。resume はモデル・effort をセッションから引き継がず、指定を省くと config 既定へサイレントに落ちる（codex-cli 0.144.4 実測。`-m` 付き resume は正しく効く。turn_context ログで裏取り可能）。`--last` は使わない（同一 Change 内に Implementer 用と Gatekeeper 用の codex セッションが並存し得るため、掴む対象が不定になる）。timeout した場合は破棄せず resume で継続する。
- `codex exec` は現在の CWD で実行し、`cd` しない。
- `codex exec` / `codex exec resume` の stdout は watchdog の transcript に直接受けず、ファイルへ隔離する: `codex exec ... > tmp/codex-<change>-<role>-round<N>.log 2>&1` の形で実行し、読み取りは `tail` と終了 marker の `grep` に絞る（ログの全量 Read はしない）。codex の stdout はビルド・テスト出力を含め数百〜数千行になり、ラウンドごとに transcript へ積むと以降の全ターンで再読され、往復の多い Change で消費が膨張することが実測されているため。ログファイルは削除せず残し、Gatekeeper / Conductor が疑義のときに参照できるようにする。

<!-- slot: 既定と異なる委譲先（外部エージェントの差し替え等）や、このプロジェクト固有の実行時の注意があれば追記する。 -->
<!-- /slot -->

## Watchdog（運転 subagent）

役割（Implementer / Gatekeeper）ごとに fresh の watchdog subagent（`sonnet` 固定）を起動し、その codex セッションの運転だけを任せる。

- 責務はこれだけに限定する: 委譲プロンプトの組み立て（Conductor から受け取った Change brief と本書の contract 定型の機械的な合成。内容の追加・削除の判断はしない）、codex exec の起動、終了・timeout の検知、resume、終了 marker と最終メッセージ・成果物パスの中継、`git status --porcelain` の生出力の取得（解釈しない）。
- 禁止: 設計質問への回答、実コード・diff・ログ本文の実読と解釈、PNG 等の証拠の目視、指摘の採否、委譲プロンプトの実質的な書き換え。
- codex が終了 marker なしで終了した・timeout した場合は「未完了」とみなし、同一セッションを resume する（fresh 再起動しない）。resume しても diff・ログに進捗が見えない状態が 2 回続いたら、事実を添えて Conductor に返して停止する。
- codex が途中で技術的質問を出して止まった場合は、内容に踏み込まず「contract に従い実コードを確認して自己解決し、Stop 級のみ marker で停止せよ」と定型で返す。STOP 系 marker は解釈せずそのまま Conductor に中継する。codex が証拠付きで委譲プロンプトの誤りを主張した場合も、watchdog は判断せず Conductor に中継する（brief の訂正は Conductor の責務）。

## 終了 marker（codex の最終出力契約）

委譲プロンプトには、最終メッセージの末尾を次のいずれか 1 行で終えることを義務付ける:

- `RESULT: COMPLETE` — 担当作業（実装・検証、または受け入れ判定）まで完了
- `RESULT: STOP_PRODUCT_DECISION` — `change/workflow.md` の判断境界で Stop に該当する仕様・UX・プロダクト判断が必要
- `RESULT: STOP_CONFLICT` — 指示・正本 docs と実コードの矛盾を検出
- `RESULT: STOP_BLOCKED` — 権限・環境の制約、または必須検証が実行不能

marker なしの終了は未完了として扱う（watchdog が resume する）。STOP 系では、判断点・確認した根拠・残った妥当案・非可逆性またはやり直しコストを本文に必須とする。watchdog は marker を `goal.md` の終了種別へ機械的に対応づけて返す（COMPLETE→completed、STOP_PRODUCT_DECISION / STOP_CONFLICT→stopped、STOP_BLOCKED→blocked、marker なし→interrupted）。

Conductor は marker を信用の根拠にしない。COMPLETE でも機械照合・テスト自走・Gatekeeper（Small は Conductor の直接照合）は従来どおり必須。

## 委譲プロンプト（Implementer 委譲）

委譲前調査（read / write サイトの全列挙等）は Claude 側で行わない。調査は codex の責務とし、`change/plan.md` の plan 書き出しも Claude 側では行わない。委譲プロンプトには次を渡す:

- 変更目的、スコープ、Acceptance、調査の参照起点（backlog 項目、関連ファイルパス、参照実装があればそのブランチ・SHA）
- 実装前判断の要求: 実装に入る前に、対象フィールド・型の全構築サイト・全 read / write サイトを列挙し、実装前判断（責務配置、踏襲する既存パターン、テスト方針）を確定してから実装することを求める。これらの実装前判断は codex 自身が `tmp/delegate-plan-<change>.md` に一時 artifact として保存し、Gatekeeper の照合元にする（`change/plan.md` の Claude 系 plan file と同じ役割を担う）
- escalation の内部解決義務: 設計質問・技術的不明点は途中で外部へ問い合わせず、実コード・正本 docs を自分で確認して解決する。必要なら両案を小さく実装・検証して証拠で選ぶ。内部で subagent を使うかは codex の裁量。`change/workflow.md` の判断境界で Stop に該当する仕様・UX・プロダクト判断だけは自己解決せず `RESULT: STOP_PRODUCT_DECISION` で停止する
- git 書き込み禁止（commit / add / reset / stash / push）。commit は Conductor が行う
- 検証の実行と結果報告の義務化
- 完了報告の要求項目: 変更ファイル一覧、実行した検証コマンドと結果、指示から外れた点・自己判断した点、commit message の草案
- 終了 marker 義務（終了 marker 節のとおり）と、最終メッセージへの集約義務: 設計質問の自己解決内容・指示からの逸脱・重要な自己判断・検証結果の要旨は、途中経過ではなく必ず最終メッセージに含めて出力させる。stdout はファイル隔離して tail しか読まないため、途中経過にのみ書かれた情報は拾われない前提で運用する
- 目視・実行でしか確定できない成果物（UI の見た目、CLI の出力等）は、証拠（レンダ画像・スクリーンショット・実行ログ等）の取得を義務化し、さらに codex 自身が画像閲覧ツールで証拠を開いて内容を確認し、確認した内容を最終メッセージで報告させる（codex exec はローカル PNG をパス指定で閲覧できる。resume 後も可。2026-07-13 実測）。証拠のパスは Gatekeeper への引き継ぎに含める
  <!-- slot: このプロジェクトで目視検証が必要な成果物と、その証拠取得手段があれば書く（例: UI 変更はオフスクリーンレンダで PNG を取得させる）。 -->
  - CLI の出力・終了コード・DB 副作用に触れる変更は、`go build -o bin/mdhop ./cmd/mdhop` 後の `bin/mdhop <args>` 実行ログ（コマンド・stdout / stderr・終了コード）を提出させる。
  - `delete --rm`, `move`, rewrite 系などの破壊的処理は、`testdata/` または `tmp/` 配下の一時 vault で実行した証拠を求める。
  <!-- /slot -->

## Gatekeeper 委譲（Gatekeeper に GPT 系を指定）

Goal 指定で Gatekeeper に GPT 系モデルを座らせられる（無指定は `models.md` の役割既定。Implementer の指定とは独立）。責務・戻り値の実行証拠・差し戻し運用は `goal.md` の Gatekeeper 節と同一で、変わるのは実体と運転だけ。

- 実装セッションとは別の fresh codex セッションとして起動する（実装セッションの resume ではない。実装文脈を引き継がない）。
- 渡すもの: Change brief（Conductor が確定した不変入力）、`tmp/delegate-plan-<change>.md` のパス、baseline HEAD SHA、証拠（PNG 等）のパス。
- 求めるもの: diff 全量の実読、brief・plan との照合、テストの再実行による裏取り、指摘の整理と受け入れ判定、`goal.md` Gatekeeper 節の実行証拠（baseline HEAD SHA、`git status --porcelain` の対象状態、commit 予定差分全体のハッシュと stat、テストコマンド・exit code・所要時間、判定、採用した指摘と対応要求、残リスク、Product Decision Ledger 候補）を含む最終報告と終了 marker。
- Claude の review lane subagent は起動できないため、codex Gatekeeper は `change/review.md` と同等のレビュー観点（correctness / 正本整合 / 変更対象の全列挙による貫通確認 / プロジェクト固有制約）を自セッション内で消化する（内部 subagent は裁量）。
- tracked file の編集禁止（Gatekeeper は修正しない。差し戻しは Conductor 経由で Implementer へ）。
- 差し戻し後の再照合は、同一 Gatekeeper セッションを session ID 明示の resume で行う。
- 視覚 Acceptance が主要件の Change では、Gatekeeper の実体に関わらず Gatekeeper が証拠 PNG を直接確認する（実装側の自己目視だけで受け入れない）。

## 委譲後（watchdog の戻り値と引き継ぎ）

- watchdog の戻り値: 終了 marker（と対応づけた終了種別）、codex の最終メッセージ全文、ログファイルパスの一覧、session ID、plan artifact / 証拠のパス、`git status --porcelain` の生出力（解釈しない）。
- 意図しない git 書き込みの検出、diff の検品は watchdog の責務ではない。Conductor の機械照合（`change/finish.md`）と Gatekeeper がそれぞれ従来どおり行う。
- commit と Product Decision Ledger / Alternative Check（`design-decision-record.md`）・docs 同期の最終責任は Conductor に残る。

## Acceptance

- 委譲プロンプトが必須要素（実装前判断の要求、escalation の内部解決義務、終了 marker 義務、証拠の取得と自己確認）を満たしていた
- watchdog が責務限定を守り、marker・最終メッセージ・ログパス・session ID・`git status --porcelain` を揃えて返した
- 受け入れ判定は Gatekeeper（Small は Conductor）が行い、commit は Conductor が完了した

## Stop Conditions

- 委譲先が起動できない、または sandbox / 権限の制約で実行できない。モデル指定の扱いを Conductor 経由でユーザーに確認する
- codex の成果物が必須要素（検証・証拠・終了 marker）を満たさないまま、再依頼 2 回で改善しない
- `RESULT: STOP_PRODUCT_DECISION` / `RESULT: STOP_CONFLICT` が返った（Conductor が `change/workflow.md` の判断境界に従って扱い、Stop 級はユーザー確認へ）
- 進捗のない resume が続き、watchdog が停止を返した
