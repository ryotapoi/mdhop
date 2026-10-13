"""Instrument isolated source copies only; binaries are never used for timings."""
from pathlib import Path
import sys
p = Path(sys.argv[1])/'internal/core/util.go'
s = p.read_text()
assert s.count('os.ReadDir(') == 1
s = s.replace('os.ReadDir(', 'a304ReadDir(')
s += '''
// CHANGE-VERIFY: Artifact-only directory traversal observation; not shipped.
func a304ReadDir(directory string) ([]os.DirEntry, error) {
    entries, err := os.ReadDir(directory)
    if path := os.Getenv("MDHOP_DIRECTORY_TRACE"); path != "" {
        file, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
        if openErr != nil { panic(openErr) }
        if _, writeErr := fmt.Fprintf(file, "%s\\t%d\\n", directory, len(entries)); writeErr != nil { panic(writeErr) }
        if closeErr := file.Close(); closeErr != nil { panic(closeErr) }
    }
    return entries, err
}
'''
p.write_text(s)
