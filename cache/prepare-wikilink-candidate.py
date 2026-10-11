#!/usr/bin/env python3
"""CHANGE-VERIFY: isolated no-wikilink build hypothesis, not a product setting."""
from pathlib import Path
import shutil
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
target = Path(sys.argv[1]).resolve()
diagnostic = "--diagnostic" in sys.argv
if diagnostic:
    subprocess.run([sys.executable, str(root / "cache/instrument-build-incremental.py"), str(target)], check=True)
else:
    target.mkdir(parents=True, exist_ok=True)
    for name in ["go.mod", "go.sum"]:
        shutil.copy2(root / name, target / name)
    for name in ["internal", "cmd"]:
        shutil.copytree(root / name, target / name, dirs_exist_ok=True)
if "--enabled" not in sys.argv:
    replacements = {
        "parse.go": [("\t\tout = append(out, parseWikiLinks(clean, lineNum, tableLines[lineNum])...)\n", ""), ("\t\tclean = maskWikiLinks(clean)\n", "")],
        "parse_frontmatter.go": [("\t\tfr.links = append(fr.links, collectFrontmatterWikilinks(key.Value, val, offset)...)\n", "")],
        "parse_reference.go": [("maskWikiLinks(stripInlineCode(raw))", "stripInlineCode(raw)")],
    }
    for name, pairs in replacements.items():
        path = target / "internal/core" / name
        source = path.read_text()
        for before, after in pairs:
            assert before in source, (name, before)
            source = source.replace(before, after)
        path.write_text(source)
if diagnostic:
    path = target / "internal/core/build_incremental.go"
    source = path.read_text().replace("ReadParseNotes,ResolvedLinks,BasenameEvaluations int", "ReadParseNotes,ResolvedLinks,BasenameEvaluations,DependenciesChecked int")
    before = "if dependencyResolution(dependency, oldMaps) != dependencyResolution(dependency, newMaps) {"
    assert before in source
    source = source.replace(before, "buildTrace.DependenciesChecked++\n" + before)
    path.write_text(source)
print(target)
