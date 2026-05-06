# ADR 0013: Frontmatter wikilink detection via raw text scan

## Status

Accepted

## Context

v0.7.0 で frontmatter 内の `[[note]]` を edge として扱う要件が出た。Obsidian は `key: "[[note]]"` (quoted) と `key: [[note]]` (bare)、配列形式 (`- "[[a]]"` / `- [[a]]`) のいずれも許容しており、mdhop もこれらを統一的に拾う必要がある。

`gopkg.in/yaml.v3` で実測すると次の挙動になる:

- `key: "[[note]]"` → `Scalar "[[note]]"`
- `key: [[note]]` → ネストした flow sequence (`[[note]]` = `[ [note] ]` と解釈される)
- `key:\n  - "[[a]]"` → block seq of scalar string
- `key:\n  - [[a]]` → block seq of flow seq of flow seq

YAML パーサの構造解釈に依存して wikilink を抽出すると、quoted / bare / 配列の 4〜5 通りに分岐が必要になり、`yaml.Node.Kind` を辿るロジックが scalar / sequence / nested sequence で繰り返される。さらに将来 YAML パーサの曖昧解釈が変わった場合に脆弱になる。

合わせて、frontmatter wikilink を本文 wikilink (`linkType="wikilink"`) と同じ linkType で扱うか、新しい linkType として分離するかの判断も必要だった。本文 wikilink と分離すると edge クエリで検索できるが、書き換え系コマンド（add / update / move / disambiguate / convert）は YAML 側の quoted / bare スタイル保持や行範囲制約が本文と異なるため、type で分岐したい場面が出てくる。

## Considered Options

- **選択肢A: YAML 構造を辿る抽出**: `yaml.Node.Kind` ごとに分岐し、scalar / sequence / nested sequence の各パターンで `[[...]]` を抽出する。本文 wikilink と同じ `linkType="wikilink"` を再利用する
- **選択肢B: frontmatter 内の生テキスト行を本文 wikilink と同じ抽出ロジックに掛ける + 新 linkType `frontmatter_wikilink` を追加**: mapping の各 (key, val) について `key.Line` から「次の `key.Line - 1`」までの行範囲を計算し、その範囲を本文 wikilink 抽出ロジック (`parseWikiLinks`) で走査する。検出された occurrence は `linkType="frontmatter_wikilink"` を付ける
- **選択肢C: B と同じ生テキスト抽出だが linkType は `wikilink` を流用**: 行範囲は同様に取り出すが、本文 wikilink と区別しない

## Decision

We will adopt 選択肢B: frontmatter 内では `yaml.Node.Kind` を辿らず、frontmatter 行範囲の生テキストに本文 wikilink 抽出ロジックを当てる。検出した occurrence は新 linkType `frontmatter_wikilink` を付ける。

行範囲は次のように決める:

- mapping `Content` を `[key0, val0, key1, val1, ...]` の形で順走査
- 各エントリ i について、`startIdx = key[i].Line`、`endIdx = (i が最終なら totalLines-1, それ以外は key[i+2].Line)` で計算（0-based / closing `---` を除外）
- `tags` キーは frontmatter wikilink 抽出から除外する（既存の `parseFrontmatterTags` でハンドル済み）

## Consequences

肯定的:

- YAML パーサの曖昧解釈に依存しないため、bare `[[a]]` がネスト flow seq として解釈されても影響を受けない
- block scalar (`key: |`) も行範囲走査で自動的にカバーされる
- 抽出ロジックは `parseWikiLinks` を再利用するため alias / subpath / vault-relative path / phantom 解決は本文 wikilink と完全に同じ振る舞いになる
- `linkType="frontmatter_wikilink"` を新規型として分離したことで、書き換え系コマンドの SQL フィルタ (`link_type IN ('wikilink', 'markdown')`) と Go コードフィルタは自然に「(2/2) で対応するまで無視」の状態になる

否定的:

- 行範囲推定は「次の `key.Line - 1`」に依存するため、`yaml.v3` が将来 `Node.Line` のセマンティクスを変えると壊れる（現状の API では妥当）
- `linkType` の文字列リテラル種類が 4 → 5 に増え、`build.go` / `resolve.go` / 書き換え系コマンドの分岐ミスリスクが上がる。これは別タスク（`linkType` を `type LinkType string` に named type 昇格）として backlog Later に積んだ
- meta テーブルと edges テーブルへの両立: bare `[[a]]` は YAML 解釈上 `val.Value` が空になるため meta テーブルには載らない（quoted のみ載る）。`--where key=[[note]]` での検索は quoted のみ。この非対称性は (2/2) または別タスクで再考する

中立的:

- (1/2) では検出と edge 化のみ行い、書き換え系は (2/2) で対応する。書き換え系コマンドの SQL/Go フィルタを意図的に拡張しないことで、(1/2) のマージで挙動破壊しない設計
- `add.go:249` / `update.go:136` の ambiguous / vault-escape 検証ガードも (1/2) では `frontmatter_wikilink` を追加せず、(2/2) で `build.go:87` と揃える方針
