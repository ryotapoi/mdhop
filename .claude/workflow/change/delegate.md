# Delegate

## Intent

execution mode が `delegate` の Change で、調査・実装を外部実装エージェントへ委譲し、1 commit 分の作業を完了させる。委譲しても Change の完了責任（diff レビュー・検証・commit・記録同期）は Change worker に残る。

## Use When

- `/goal` の呼び出し文で execution mode `delegate` が指定されている
- 単発 Change でユーザーが委譲を明示した

## Delegation Target

<!-- slot: 委譲先の実装エージェントと呼び出し方を書く（起動コマンド、モデル・sandbox 設定、実行時の注意。例: codex exec の起動形）。 -->
- 委譲先は Codex CLI（GPT-5.5）。実装は `codex exec -m gpt-5.5 -s workspace-write "<prompt>" </dev/null` を Bash で実行する（`timeout: 600000`）。調査だけなら `-s read-only` または `codex` skill を使う。
- Bash 呼び出しは `dangerouslyDisableSandbox: true` で実行する。codex 自身が OS sandbox を張るため、Claude Code の sandbox と二重になると起動に失敗する。
- resume 時は `-s` が使えないため `codex exec resume --last -c sandbox_mode=workspace-write ...` を使う。
- `codex exec` は現在の CWD で実行し、`cd` しない。
- Change worker（subagent）から `codex exec` を実行できない場合は、事実を Goal main に返し、Goal main が直接委譲を実行する。
<!-- /slot -->

## 委譲前調査

委譲プロンプトの質は書き手の能力ではなく、書く前の調査の深さで決まる。対象コードだけ読んで書かず、以下を実コードで確認してから設計制約を書く。

- テストが対象の状態・データをどう使うか（テスト専用の構築経路、直接変更の有無）
- 対象フィールド・派生値の全 read / write サイト（スコープは全列挙する。実装者が列挙外を補完することを期待しない）
- computed property 等に隠れた依存（見た目より広い観測・依存がないか）
- 踏襲すべき既存パターン（あれば「このファイルのこの形と同型に」と指差す）
- 構築面の呼び出し（init / Preview / テストヘルパ等、フィールド追加で壊れる箇所）
- 並行性の文脈（共有 mutable state を実装者が導入しないか）

## 委譲プロンプトの必須要素

- 変更目的、スコープの全列挙、踏襲パターン、設計制約
- git 書き込み禁止（commit / add / reset / stash / push）。commit は Change worker が行う
- 検証の実行と結果報告の義務化
- 完了報告の要求項目: 変更ファイル一覧、実行した検証コマンドと結果、指示から外れた点・自己判断した点
- 目視・実行でしか確定できない成果物（UI の見た目、CLI の出力等）は、実装者に証拠（レンダ画像・スクリーンショット・実行ログ等）の取得と提出を義務化する。証拠なしの完了報告を受け入れない
  <!-- slot: このプロジェクトで目視検証が必要な成果物と、その証拠取得手段があれば書く（例: UI 変更はオフスクリーンレンダで PNG を取得させる）。 -->
  - CLI の出力・終了コード・DB 副作用に触れる変更は、`go build -o bin/mdhop ./cmd/mdhop` 後の `bin/mdhop <args>` 実行ログ（コマンド・stdout / stderr・終了コード）を提出させる。
  - `delete --rm`, `move`, rewrite 系などの破壊的処理は、`testdata/` または `tmp/` 配下の一時 vault で実行した証拠を求める。
  <!-- /slot -->

## 委譲中の応答

- 実装者からの設計質問・escalation には推測で即答しない。自分で実コードを確認してから答えるか、必要なら「両案を実装・検証して証拠付きで比較」を実装者に返す。
- 実装者が証拠付きで委譲プロンプトの誤りを指摘した場合は、握りつぶさず事実を確認して指示を訂正する。

## 委譲後（省略不可）

- 実装者の自己報告がどれだけ整っていても、diff レビューは省略しない。テスト green でも捕まらない欠陥（並行性、テスト空白域の破壊等）はここでしか見つからない。
- `git status` で意図しない git 書き込み（stage・commit 等）がないか裏取りする。
- formatter hook 等の自動整形が diff に混ざる場合、実装者の逸脱と誤判定しない。
- diff レビュー後の検証・レビュー・commit・記録同期は、通常どおり `change/verify.md` / `change/review.md` / `change/finish.md` に従う。

## Acceptance

- 委譲プロンプトが必須要素を満たしていた
- 実装者の成果物を diff レビューと検証で裏取りした
- Change worker として commit と記録同期を完了した（または呼び出し元へ停止理由を返した）

## Stop Conditions

- 委譲先が起動できない、または sandbox / 権限の制約で実行できない。Goal 実行中は事実を Goal main に返し、Goal main が直接委譲を実行するか execution mode の扱いを判断する
- 実装者の成果物が必須要素（検証・証拠）を満たさないまま、再依頼 2 回で改善しない
- 委譲中の escalation で、その時点の情報では適切に決められない重要な仕様・UX・プロダクト判断が必要になった
