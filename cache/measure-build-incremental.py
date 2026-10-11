#!/usr/bin/env python3
"""Compare baseline/full/auto CLI builds on isolated, warm-cache vaults."""
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

baseline, candidate = map(lambda arg: str(Path(arg).resolve()), sys.argv[1:3])
trace = str(Path(sys.argv[3]).resolve()) if len(sys.argv) > 3 else None
root = Path(__file__).resolve().parents[1]
work = Path(tempfile.mkdtemp(prefix="mdhop-build-"))
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
    for i in range(count):
        target = (i + 1) % count
        link = f"[[N{target:04d}]] [[Future]]" if wiki else f"[next](/d{target%20:02d}/N{target:04d}.md) [future](/Future.md)"
        content = f"---\nrank: {i}\n---\n{link} #topic\n"
        content += padding * max(0, (body - len(content)) // len(padding))
        write(vault / f"d{i%20:02d}/N{i:04d}.md", content)


def run_case(name, count, body, wiki, change, rate, representative=False):
    if time.monotonic() - started > 480:
        raise RuntimeError("run budget exceeded")
    case = work / name
    vault = case / "vault"
    vault.mkdir(parents=True)
    if representative:
        for source in ["docs", "decisions"]:
            shutil.copytree(root / source, vault / source)
        shutil.copy2(root / "README.ja.md", vault / "README.ja.md")
    else:
        synth(vault, count, body, wiki)
    db = {mode: case / f"{mode}.sqlite" for mode in ["baseline", "full", "auto"]}
    command(candidate, vault, db["auto"], True)
    snapshot = case / "initial.sqlite"
    shutil.copy2(db["auto"], snapshot)
    paths = sorted(vault.rglob("*.md"))
    note_count = len(paths)
    body_bytes = sum(path.stat().st_size for path in paths)
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
    # Warm each path before paired measurement, restore the initial auto index.
    for mode in db:
        if mode == "auto":
            shutil.copy2(snapshot, db[mode])
        command(baseline if mode == "baseline" else candidate, vault, db[mode], mode == "full")
    for repetition in range(3):
        modes = ["baseline", "full", "auto"] if repetition % 2 == 0 else ["auto", "full", "baseline"]
        for mode in modes:
            if mode == "auto":
                shutil.copy2(snapshot, db[mode])
            elapsed, stderr = command(baseline if mode == "baseline" else candidate, vault, db[mode], mode == "full")
            samples[mode].append(elapsed)
        expected = semantics(db["full"])
        assert semantics(db["auto"]) == expected, name + " auto semantics"
        assert semantics(db["baseline"]) == expected, name + " baseline semantics"
    telemetry = None
    if trace:
        shutil.copy2(snapshot, db["auto"])
        _, stderr = command(trace, vault, db["auto"])
        telemetry = [json.loads(line[len("BUILD_TRACE "):]) for line in stderr.splitlines() if line.startswith("BUILD_TRACE ")]
    value = {"case": name, "notes": note_count, "body_bytes": body_bytes, "basename_dependencies": wiki, "change": change, "changed": amount, "rate": rate, "samples_ms": samples, "summary_ms": {mode: {"median": statistics.median(values), "min": min(values), "max": max(values)} for mode, values in samples.items()}, "db_bytes": {mode: path.stat().st_size for mode, path in db.items()}, "trace": telemetry}
    print(json.dumps(value), flush=True)
    return value


results = [run_case("representative-docs", 0, 0, False, "body", 0, True)]
for count in [64, 1500]:
    for body in [768, 8192]:
        for wiki in [False, True]:
            for rate in [0, 0.01, 0.5]:
                name = f"n{count}-b{body}-{'wiki' if wiki else 'path'}-body-{rate}"
                results.append(run_case(name, count, body, wiki, "body", rate))
for wiki in [False, True]:
    for change in ["add", "delete", "move"]:
        name = f"n1500-b8192-{'wiki' if wiki else 'path'}-{change}"
        results.append(run_case(name, 1500, 8192, wiki, change, 0.01))
report = {"environment": {"platform": platform.platform(), "go": subprocess.check_output(["go", "version"], text=True).strip(), "cpu": platform.processor(), "sqlite_module": "modernc.org/sqlite v1.29.0", "cache": "warm OS cache; no flush; distinct explicit external DBs", "baseline": baseline, "candidate": candidate, "trace": trace}, "duration_seconds": time.monotonic() - started, "results": results}
(work / "product.json").write_text(json.dumps(report, indent=2) + "\n")
print(f"completed: {work / 'product.json'}", flush=True)
