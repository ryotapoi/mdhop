# Delegate

## Intent

execution mode `delegate` 用。Implementer は Claude subagent ではなく外部実装エージェントとし、Orchestrator が Change 単位で直接委譲して 1 commit 分の作業を完了させる。実装エージェントは調査・実装計画・実装を一体で行う（内部で subagent を使うかは実装エージェント側の裁量とし、こちらから分業を指示しない）。委譲しても Change の完了責任（diff レビュー・裏取り・検証・commit・記録同期）は Orchestrator に残る。

## Use When

- Goal 経由の Change で、execution mode が `delegate`
- 使い分けの目安は `goal.md` の execution mode の項に従う（重要部分は `solo`、それ以外で Claude 側の使用量を抑えたい Goal は `delegate`）
- Goal 途中で個別 Change の Intake が High-risk になった場合は、その Change だけ `solo`（fresh Implementer 起動）へ切り替えてよい。切り替えは mode 固定の例外として最終報告に明記する。切り替えても進められない場合は停止してユーザーに確認する

実装対象がない Change（docs / backlog 整理のみ等）は委譲の対象外とし、Orchestrator が直接編集してよい。これはこの workflow からの逸脱ではない。

## 呼び出し方

既定の委譲先は Codex CLI。Implementer のモデルは `gpt-5.6-terra` を既定とし、ユーザーが Goal 指定でモデルを明示した場合（例: `gpt-5.6-sol`）はそちらを優先する。呼び出し方は全プロジェクト共通:

- 実装は `codex exec -m <Implementer のモデル> -s workspace-write "<prompt>" </dev/null` を Bash で実行する（`timeout: 600000`）。`-m` には上記で確定したモデル（既定 `gpt-5.6-terra`、ユーザー明示があればそのモデル）を渡す。調査だけなら `-s read-only` または `codex` skill を使う。
- Bash 呼び出しは `dangerouslyDisableSandbox: true` で実行する。codex 自身が OS sandbox を張るため、Claude Code の sandbox と二重になると起動に失敗する。
- resume 時は `-s` が使えないため `codex exec resume --last -c sandbox_mode=workspace-write ...` を使う。timeout した場合は破棄せず resume で継続する。
- `codex exec` は現在の CWD で実行し、`cd` しない。

<!-- slot: 既定と異なる委譲先（外部エージェントの差し替え等）や、このプロジェクト固有の実行時の注意があれば追記する。 -->
<!-- /slot -->

## 委譲プロンプト

委譲前調査（read / write サイトの全列挙等）は Claude 側で行わない。調査は実装エージェントの責務とし、`change/plan.md` の plan 書き出しも行わない（実装計画は実装エージェントの成果物として委譲の中で作られる）。委譲プロンプトには次を渡す:

- 変更目的、スコープ、Acceptance、調査の参照起点（backlog 項目、関連ファイルパス、参照実装があればそのブランチ・SHA）
- 実装前判断の要求: 実装に入る前に、対象フィールド・型の全構築サイト・全 read / write サイトを列挙し、実装前判断（責務配置、踏襲する既存パターン、テスト方針）を確定してから実装することを求める
- git 書き込み禁止（commit / add / reset / stash / push）。commit は Orchestrator が行う
- 検証の実行と結果報告の義務化
- 完了報告の要求項目: 変更ファイル一覧、実行した検証コマンドと結果、指示から外れた点・自己判断した点
- 目視・実行でしか確定できない成果物（UI の見た目、CLI の出力等）は、実装エージェントに証拠（レンダ画像・スクリーンショット・実行ログ等）の取得と提出を義務化する。証拠なしの完了報告を受け入れない
  <!-- slot: このプロジェクトで目視検証が必要な成果物と、その証拠取得手段があれば書く（例: UI 変更はオフスクリーンレンダで PNG を取得させる）。 -->
  - CLI の出力・終了コード・DB 副作用に触れる変更は、`go build -o bin/mdhop ./cmd/mdhop` 後の `bin/mdhop <args>` 実行ログ（コマンド・stdout / stderr・終了コード）を提出させる。
  - `delete --rm`, `move`, rewrite 系などの破壊的処理は、`testdata/` または `tmp/` 配下の一時 vault で実行した証拠を求める。
  <!-- /slot -->

## 委譲中の応答

- 実装エージェントからの設計質問・escalation には推測で即答しない。自分で実コードを確認してから答えるか、必要なら「両案を実装・検証して証拠付きで比較」を実装エージェントに返す。
- 実装エージェントが証拠付きで委譲プロンプトの誤りを指摘した場合は、握りつぶさず事実を確認して指示を訂正する。

## 委譲後（省略不可）

- 実装エージェントの自己報告がどれだけ整っていても、diff レビューは省略しない。テスト green でも捕まらない欠陥（並行性、テスト空白域の破壊等）はここでしか見つからない。
- `git status` で意図しない git 書き込み（stage・commit 等）がないか裏取りする。
- formatter hook 等の自動整形が diff に混ざる場合、実装エージェントの逸脱と誤判定しない。
- diff レビュー後の検証・レビュー・commit・記録同期は Orchestrator が直接行い、通常どおり `change/verify.md` / `change/review.md` / `change/finish.md` に従う。実装エージェントの自己報告は正確とは限らない。テスト結果・変更ファイルは自分で再確認する。
- Product Decision Ledger / Alternative Check（`design-decision-record.md`）の判断と記録、docs 同期は Orchestrator の責務のまま残り、通常どおり `change/workflow.md` と `design-decision-record.md` に従う。実装エージェントに委ねない。

## Acceptance

- 委譲プロンプトが必須要素（実装前判断の要求を含む）を満たしていた
- 実装エージェントの成果物を diff レビューと検証で裏取りした
- Orchestrator として commit と記録同期を完了した

## Stop Conditions

- 委譲先が起動できない、または sandbox / 権限の制約で実行できない。execution mode の扱いをユーザーに確認する
- 実装エージェントの成果物が必須要素（検証・証拠）を満たさないまま、再依頼 2 回で改善しない
- 委譲中の escalation で、`change/workflow.md` の判断境界で Stop に該当する重要な仕様・UX・プロダクト判断が必要になった
