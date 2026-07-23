# Goal Review 実行手順

Goal Review の実行手順の正本。実行するのは Conductor で、Goal Review を実施する直前に毎回このファイルを読んでから reviewer を起動する（分割レビューの 2 回目以降も同じ）。規定が古く見えても（workflow 変更の commit が履歴にある等）、このファイルの現在の内容を正とする。

実施タイミング（いつ・どの range をレビューするか）は `goal.md` の Goal Review 節を正とする。

## reviewer の選定

- reviewer は実装文脈を引き継がない fresh reviewer とし、常に次の 2 本を回す。選定と起動は Conductor 自身が行い、subagent に委ねない。その Goal 内で起動した Gatekeeper を Goal Review の reviewer に再利用してはならない（個々の Change の受け入れ判定者であり、commit 間の統合を見る立場と分けるため）。Conductor 自身も reviewer にしない。
  - **自系統 reviewer**: `codex-fresh-review` skill に依頼する（実装文脈を引き継がない fresh Review subagent）。使うモデルは `models.md` の GPT 系 Goal Review reviewer 既定とする。
  - **他系統 reviewer**: `claude-fresh-review` skill に、レビューモデルとして `models.md` の Goal Review reviewer 既定（Claude 系）を明示して依頼する（別系統の Claude Code。低レベル transport は `claude-review-request` 側に委譲する）。
- 2 本とも PASS 相当であることを Goal Review 通過条件とする。回数上限と PASS / 指摘 / 上限到達は reviewer ごとに数え、最終報告で個別に分かるようにする。
- 2 本には同じ range `<review_start>..<review_end>` を渡す。修正は Codex 側（Conductor 経由の Implementer）が行い、reviewer は指摘を返すだけで実装しない。Herdr は明示的に使う場合だけの transport とし、workflow の通常経路では tmux を使う。

## 実行

- 実行直前に `review_start = review_cursor`、`review_end = 現在の HEAD の実 SHA` を確定し、1 回の review 中は `<review_start>..<review_end>` を動かさない。
- レビュー依頼には「変更したフィールド・型・メッセージについて、全構築サイト・全 read / write サイトを列挙して貫通漏れがないか確認する」観点を含める（複数ある組み立て経路の一部だけ修正される欠陥クラスに直効するため）。
- **finding の合流**: Conductor は 2 本の finding を機械的に束ねるだけにする（重複の統合、対象ファイル・行での並べ替え、reviewer 名の併記）。finding の要約・取捨選択・言い換えはしない。両者の指摘が割れた場合（片方が MUST、もう片方が問題なしとした等）は Conductor が判断せず、割れている事実と両者の根拠を添えてユーザー判断に回す。
- 指摘対応は別 commit として作成し、対応 commit を含む range で再レビューする。follow-up review でも実行直前に新しい `review_end` を取り直す。

## 回数上限

- 各レビュー単位につき reviewer を呼ぶ回数は、初回を含めて合計最大 3 回（reviewer ごとに数える）。`Review 1 -> Fix 1 -> Review 2 -> Fix 2 -> Review 3 -> Fix 3` まで行ったら Review 4 は行わない。Review 3 後の Fix 3 は未レビューの最終修正になるため、同じ review 単位を上限到達として打ち切り、Goal 作業は続ける。
- **レビュー上限超過時の扱い**: 上限に達した review 単位は上のとおり打ち切り、`レビュー上限超過` として報告する。上限を自動で延長したり、reviewer を自動で差し替えたりしない。停止してユーザーに承認を求める場合、選択肢としてモデル / effort の引き上げや別系統の reviewer への差し替えを提示してよいが、実行はユーザーの承認後に限る。

## 実行時記録

- 各ラウンド（同じ range に対する 2 本の reviewer の 1 セット）が終わるたびに、その直後に `tmp/workflow/<scope>/goal-review-log.md` へ 1 エントリを追記する。あとでまとめて書かない。
- 1 エントリの項目: ラウンド番号、対象 range（`<review_start>..<review_end>` の実 SHA）、自系統 reviewer（実際に呼び出した skill 名と verdict）、他系統 reviewer（同）、指摘件数（MUST は一行タイトルを添える）。指摘ゼロの reviewer も「指摘なし」と明記する。指摘の詳細本文はこのログに書かない（詳細は review 結果と対応 commit が持つ）。
- skill 名は実際に呼び出した名前をそのまま書く。規定の skill を使わず別の手段で代替した場合は、その事実をログに書く（規定 reviewer を実施できない状態なので Stop Conditions の対象）。
- Final Report の Goal Review 結果はこのログから転記する。記憶で再構成しない。
