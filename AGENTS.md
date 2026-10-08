# mdhop

mdhop の目的・制約は `docs/requirements.md` にある。作業開始時に `docs/development.md` の言語・作業規則を読む。

## タスク別の正本

タスクに必要な文書だけを読む。推測で済ませず、判断に影響する正本を確認する。

- 目的・要件・出力の安定性: `docs/requirements.md`
- CLI の操作: `README.ja.md` と `mdhop <command> --help`。実装・契約の変更では `cmd/mdhop/` の対象コマンドとテストを読む
- DB schema、query、link resolve、path: `internal/core/` の対象実装とテスト、および関連する有効な ADR
- 責務配置・依存方向・SQL の所有: `decisions/0032-module-and-sql-ownership.md`
- 情報の配置: `docs/rules/information-management.md`
- 検証: `docs/verification.md`
- versioning・release: `docs/release.md`
- 依存・Go version の更新と配布: `docs/licensing.md`
- 設計判断: `decisions/` の `status: current` の ADR。過去の判断を調べる場合は `status: superseded` の ADR
- 未着手・未リリースのタスク: `backlog/backlog.md`
