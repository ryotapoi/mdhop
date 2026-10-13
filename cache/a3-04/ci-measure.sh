#!/usr/bin/env bash
set -euo pipefail
base=54fb0473ddfd9cf2695c40f20cfcf040570afa51
workspace="$RUNNER_TEMP/a3-04-measure"
mkdir -p "$workspace/before" "$workspace/after"
git archive "$base" | tar -x -C "$workspace/before"
git archive HEAD | tar -x -C "$workspace/after"
for variant in before after; do
  (cd "$workspace/$variant" && go build -o "$workspace/$variant-bin" ./cmd/mdhop)
  cp -R "$workspace/$variant" "$workspace/$variant-probe"
  python3 cache/a3-04/instrument.py "$workspace/$variant-probe"
  (cd "$workspace/$variant-probe" && go build -o "$workspace/$variant-probe-bin" ./cmd/mdhop)
done
python3 cache/a3-04/measure.py "$workspace/before-bin" "$workspace/after-bin" "$workspace/results" "$workspace/before-probe-bin" "$workspace/after-probe-bin"
