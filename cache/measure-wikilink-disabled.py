#!/usr/bin/env python3
"""CHANGE-VERIFY: compare enabled/full and isolated no-wikilink hypothesis."""
import json
import os
from pathlib import Path
import platform
import shutil
import sqlite3
import statistics
import subprocess
import sys
import tempfile
import time

enabled, disabled, enabled_trace, disabled_trace = [str(Path(arg).resolve()) for arg in sys.argv[1:5]]
repetitions = int(sys.argv[5]) if len(sys.argv) > 5 else 5
smoke = "--smoke" in sys.argv
root = Path(__file__).resolve().parents[1]
work = Path(tempfile.mkdtemp(prefix="mdhop-wikilink-"))
print(f"artifacts: {work}", flush=True)
started = time.monotonic()
# Isolated <=1500 notes/12 MiB body per case, serial commands, <=60 s per
# command, <=8 minutes for the run. Failure/output mismatch stops immediately.

def command(binary, vault, db, full=False):
    args = [binary, "build", "--vault", str(vault), "--db", str(db)]
    if full:
        args.append("--full")
    start = time.perf_counter_ns()
    result = subprocess.run(args, capture_output=True, timeout=60)
    elapsed = (time.perf_counter_ns() - start) / 1e6
    if result.returncode or result.stdout:
        raise RuntimeError((args, result.returncode, result.stdout, result.stderr))
    return elapsed, result.stderr.decode()


def semantics(path):
    db = sqlite3.connect(path)
    queries = [
        "SELECT node_key,type,name,path,exists_flag,mtime,lines FROM nodes ORDER BY node_key",
        "SELECT s.node_key,t.node_key,e.link_type,e.raw_link,e.frontmatter_key,e.reference_target,e.in_table,e.subpath,e.line_start,e.line_end FROM edges e JOIN nodes s ON s.id=e.source_id JOIN nodes t ON t.id=e.target_id ORDER BY s.node_key,t.node_key,e.link_type,e.raw_link,e.frontmatter_key,e.reference_target,e.in_table,e.subpath,e.line_start,e.line_end",
        "SELECT n.node_key,m.key,m.value,m.line,m.sort_value,m.value_type FROM meta m JOIN nodes n ON n.id=m.node_id ORDER BY n.node_key,m.key,m.value,m.line,m.sort_value,m.value_type",
    ]
    value = [db.execute(query).fetchall() for query in queries]
    db.close()
    return value


def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)


def synth(vault, count, body, wiki):
    padding = ("ordinary text " * 10 + "\n")
    write(vault / "asset.svg", "<svg/>\n")
    for i in range(count):
        target = (i + 1) % count
        link = f"[[N{target:04d}]] [[Future]]" if wiki else f"[next](/d{target%20:02d}/N{target:04d}.md) [future](/Future.md)"
        content = f"---\nrank: {i}\n---\n{link} ![image](/asset.svg) #topic\n"
        content += padding * max(0, (body - len(content)) // len(padding))
        write(vault / f"d{i%20:02d}/N{i:04d}.md", content)


def run_case(name, count, body, change, rate, representative=False, raw_path=False):
    if time.monotonic() - started > 480:
        raise RuntimeError("run budget exceeded")
    case = work / name
    vault = case / "vault"
    vault.mkdir(parents=True)
    selected, excluded = [], []
    if representative:
        for folder in ["docs", "decisions"]:
            for source in sorted((root / folder).glob("*.md")):
                relative = str(source.relative_to(root))
                if "[[" in source.read_text():
                    excluded.append(relative)
                    continue
                selected.append(relative)
                destination = vault / relative
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source, destination)
    else:
        synth(vault, count, body, False)
    if raw_path:
        write(vault / "mdhop.toml", '[meta]\nlink_keys = ["related"]\n')
        for path in vault.rglob("*.md"):
            source = path.read_text().replace("---\nrank:", "---\nrelated: Future\nrank:", 1)
            path.write_text(source)
    paths = sorted(vault.rglob("*.md"))
    assert all("[[" not in path.read_text() for path in paths)
    note_count = len(paths)
    body_bytes = sum(path.stat().st_size for path in paths)
    asset_count = len([p for p in vault.rglob("*") if p.is_file() and p.suffix != ".md" and p.name != "mdhop.toml"])
    db = {mode: case / f"{mode}.sqlite" for mode in ["full", "enabled", "disabled"]}
    command(enabled, vault, db["enabled"], True)
    snapshot = case / "initial.sqlite"
    shutil.copy2(db["enabled"], snapshot)
    # Both parsers have identical meaning on these no-wikilink inputs.
    command(disabled, vault, db["disabled"], True)
    assert semantics(db["disabled"]) == semantics(snapshot), name + " initial disabled semantics"
    amount = max(1, int(note_count * rate)) if rate else 0
    if change == "body":
        for path in paths[:amount]:
            with path.open("a") as file:
                file.write("\nchanged text #edited\n")
    elif change == "add":
        for i in range(amount):
            write(vault / ("Future.md" if i == 0 else f"Added{i}.md"), "# New\n")
    elif change == "delete":
        for path in paths[:amount]:
            path.unlink()
    elif change == "move":
        for path in paths[:amount]:
            destination = vault / "moved" / path.name
            destination.parent.mkdir(exist_ok=True)
            path.rename(destination)
    samples = {mode: [] for mode in db}
    binaries = {"full": enabled, "enabled": enabled, "disabled": disabled}
    for mode in db:
        shutil.copy2(snapshot, db[mode])
        command(binaries[mode], vault, db[mode], mode == "full")
    for repetition in range(repetitions):
        modes = ["full", "enabled", "disabled"] if repetition % 2 == 0 else ["disabled", "enabled", "full"]
        for mode in modes:
            shutil.copy2(snapshot, db[mode])
            elapsed, _ = command(binaries[mode], vault, db[mode], mode == "full")
            samples[mode].append(elapsed)
        expected = semantics(db["full"])
        assert semantics(db["enabled"]) == expected, name + " enabled semantics"
        assert semantics(db["disabled"]) == expected, name + " disabled semantics"
    # Candidate full uses the same effective disabled interpretation.
    disabled_full = case / "disabled-full.sqlite"
    command(disabled, vault, disabled_full, True)
    assert semantics(disabled_full) == expected, name + " disabled full semantics"
    raw_targets = []
    if raw_path:
        connection = sqlite3.connect(db["full"])
        raw_targets = connection.execute("SELECT DISTINCT t.type,t.path FROM edges e JOIN nodes t ON t.id=e.target_id WHERE e.link_type='frontmatter_path'").fetchall()
        connection.close()
        assert raw_targets == ([('note', 'Future.md')] if change == "add" else [('phantom', None)]), (name, raw_targets)
    telemetry = {}
    for mode, binary in [("enabled", enabled_trace), ("disabled", disabled_trace)]:
        shutil.copy2(snapshot, db[mode])
        _, stderr = command(binary, vault, db[mode])
        telemetry[mode] = [json.loads(line[len("BUILD_TRACE "):]) for line in stderr.splitlines() if line.startswith("BUILD_TRACE ")]
        assert semantics(db[mode]) == expected, name + " trace semantics"
    value = {"case": name, "notes": note_count, "assets": asset_count, "body_bytes": body_bytes, "selected": selected, "excluded_wiki_literal": excluded, "raw_path": raw_path, "raw_targets": raw_targets, "change": change, "changed": amount, "rate": rate, "samples_ms": samples, "summary_ms": {mode: {"median": statistics.median(values), "min": min(values), "max": max(values)} for mode, values in samples.items()}, "trace": telemetry, "semantics_equal": True, "disabled_full_equal": True}
    print(json.dumps(value), flush=True)
    return value

results = [run_case("representative-body-0", 0, 0, "body", 0, True)]
if not smoke:
    for change, rate in [("body", .01), ("body", .5), ("add", .01), ("delete", .01), ("move", .01)]:
        results.append(run_case(f"representative-{change}-{rate}", 0, 0, change, rate, True))
    for count, body in [(64, 768), (1500, 8192)]:
        for change, rate in [("body", 0), ("body", .01), ("body", .5), ("add", .01), ("add", .5), ("delete", .01), ("delete", .5), ("move", .01), ("move", .5)]:
            results.append(run_case(f"n{count}-b{body}-{change}-{rate}", count, body, change, rate))
    for change, rate in [("body", .01), ("add", .01), ("delete", .01), ("move", .01)]:
        results.append(run_case(f"raw-basename-{change}", 64, 768, change, rate, raw_path=True))
report = {"environment": {"platform": platform.platform(), "go": subprocess.check_output(["go", "version"], text=True).strip(), "cpu": platform.processor(), "cache": "warm OS cache; no flush; serial alternating pairs; initial DB restored outside timer", "source_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip(), "repetitions": repetitions, "binaries": {str(Path(p).name): __import__("hashlib").sha256(Path(p).read_bytes()).hexdigest() for p in [enabled, disabled, enabled_trace, disabled_trace]}}, "duration_seconds": time.monotonic() - started, "results": results}
(work / "product.json").write_text(json.dumps(report, indent=2) + "\n")
print(f"completed: {work / 'product.json'}", flush=True)
