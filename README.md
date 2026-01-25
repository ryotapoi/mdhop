# mdhop 資料セット

mdhop の仕様は `docs/` に整理しています。全体の目次は `docs/README.md` にあります。

1. `docs/architecture/01-concept.md` : コンセプト・狙い・思想・ユースケース
2. `docs/architecture/02-requirements.md` : 要件（機能/非機能/運用）
3. `docs/architecture/03-data-model.md` : 現段階で固まっているデータモデルと詳細
4. `docs/external/overview.md` : 外部仕様（統合版）
5. `docs/adr/0001-tech-stack.md` : 技術スタックの方針

## 開発用コマンド

```
go test ./...
go build -o bin/mdhop ./cmd/mdhop
```
