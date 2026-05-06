# Goal State

status: running

## Current scope

`backlog/backlog.md` の v0.7.0 の全タスク完了。

## Last completed loop

2026-05-06: `internal/core/init_meta.go`（578 行）の 3 責務分離を完了。
- 純粋なファイル分割。シンボル名・関数本体・シグネチャは保持
- `init_meta.go`（130 行）: `mergeMetaConfig` / `InitMetaOptions` / `InitMetaResult` / `InitMeta`（マージとエントリポイント）
- `init_meta_yaml.go`（278 行）: `presetEntry` / `presetMetaTypes` / `orderedKeys` / `buildMetaYAMLNode` / `formatSamples` / `buildTypeComment` / `buildOrderedValueNode` / `mergeIntoExistingYAML` / `marshalYAML` / `generateMetaYAML`（YAML 生成）
- `init_meta_infer.go`（183 行）: 推論定数 / `keyStats` / `InferredMeta` / `looksLikeDate|Number|Semver` / `inferType` / `skipKeys` / `scanMetaTypes`（型推論+走査）
- `go vet ./...` / `go test ./...` / `go build ./...` 全部グリーン
- レビュー: Intake 非 Small だが純粋リファクタなので self-check で完了

## Skipped tasks

なし

## Last verification

`go build ./...` / `go vet ./...` / `go test ./...` 全部グリーン。

## Next hint

次タスクは backlog v0.7.0 の上から: `internal/core/move_dir.go` の `MoveDir` ロールバックパスのテスト追加（`MoveDir` 分解の前提として、ロールバック経路 585/607/616 行の回帰検出網を先に張る）。
