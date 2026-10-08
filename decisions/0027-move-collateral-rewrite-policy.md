---
status: current
---

# ADR 0027: move のコラテラル書き換え方針

## 背景

move で basename 衝突が生じると、第三者 note のリンクが曖昧になったり、移動 note 自身の outgoing link の解決先が変わったりする。移動前 DB には元の target があるため、ユーザーに先に手作業で直させず意味を維持できる。ADR 0008 の自動書き換えを維持し、外部ファイルの stale 検査拡大は ADR 0012 の撤廃判断に合わせる。

## 判断と理由

- 移動後に曖昧になる第三者 note の basename link と、移動により解決先が変わる移動 note 自身の outgoing basename link は、自動で full path へ書き換える。
- 書き換え先は移動前 DB の解決先から決める。移動後に同じ名前で再解決して別 target を選ぶことを避け、元のリンクの意味を維持するためである。
- これらのケースを一律エラーで拒否したり、`--force` でユーザーに選ばせたりしない。意味が既知のリンクは自動で保護でき、add の auto-disambiguate と同じ方針にできる。
- incoming / collateral の外部書き換え対象には DB mtime と disk mtime の stale 検査を行わない。Obsidian / iCloud による内容を伴わない mtime 更新で連続 move が止まることを避ける。移動するファイル自身の stale 検査は維持する。

## 帰結

以前は拒否した衝突を move で処理できる。full path は basename より冗長になるが、意味の保持を優先する。外部ファイルが実際に編集され行位置がずれた場合、置換が no-op でも DB edge が更新される場合がある。この不整合は build で復旧する。置換成功の追跡と miss 時の DB 更新抑止は採用せず、現在の保証には含めない。
