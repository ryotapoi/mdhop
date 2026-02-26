---
tags: [mytag]
---

Path links to unique notes:
- [[sub/B]]
- [text](sub/C.md)
- [noext](sub/B)

Path links to ambiguous notes:
- [[dir1/M]]

Path links to assets:
- [[images/photo.png]]

Ambiguous asset:
- [[assets1/icon.png]]

Subpath link:
- [[sub/B#Heading]]

Alias link:
- [[sub/B|alias]]

Markdown with fragment:
- [text](sub/B.md#section)

Self link:
- [[#Heading]]

Basename link (should be untouched):
- [[B]]

Inline code: `[[sub/B]]` should be skipped.

Tag: #mytag
