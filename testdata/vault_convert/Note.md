---
tags: [test]
---

# Note

## Wikilinks (for --to markdown tests)

- [[Target]]
- [[Target#Heading]]
- [[Target|custom alias]]
- [[sub/Deep]]
- [[sub/Deep|custom]]
- [[./Sibling]]
- [[photo.png]]
- [[assets/photo.png]]
- [[#Section]]
- [[#Section|custom section]]

## Markdown links (for --to wikilink tests)

- [Target](Target.md)
- [Target](Target.md#Heading)
- [custom alias](Target.md)
- [Deep](sub/Deep.md)
- [custom](sub/Deep.md)
- [Sibling](./Sibling.md)
- [photo.png](photo.png)
- [photo](assets/photo.png)
- [#Section](#Section)
- [custom section](#Section)
- [Google](https://google.com)

## Code blocks (should not be converted)

```
[[Target]] should not change
[Link](Link.md) should not change
```

## Inline code (should not be converted)

Inline `[[Target]]` and `[Link](Link.md)` should not change.

## Embed

![[photo.png]]
![photo](photo.png)

## Note.v1 test

- [[Note.v1]]
- [Note.v1](Note.v1.md)
