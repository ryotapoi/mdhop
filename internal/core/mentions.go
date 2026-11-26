package core

import (
	"cmp"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// MentionsEntry selects exactly one note file or phantom name.
type MentionsEntry struct {
	File    string
	Phantom string
}

// MentionsOptions controls name matching, result filtering, and the
// per-direction result size. Partial disables the Latin letter boundary check.
// Path keeps notes matching any glob (phantoms have no path and are kept);
// NoPhantom drops phantoms. Where keeps only notes matching the meta
// conditions, so it also drops phantoms, as in query. Filters apply before
// Limit. A nil Limit returns all. Relations selects the directions to
// compute and return; nil selects both for a note entry and incoming for a
// phantom entry. Sort orders each direction after filtering and before Limit
// as search does ("key" ascending, "-key" descending, "" default order);
// results without the key, including all phantoms, go last.
type MentionsOptions struct {
	Partial   bool
	Limit     *int
	Filter    *QueryFilter
	Path      []string
	NoPhantom bool
	Where     *WhereClause
	Relations []string
	Sort      string
}

// Mention directions selectable by MentionsOptions.Relations.
const (
	MentionsIncoming = "incoming"
	MentionsOutgoing = "outgoing"
)

// Mention is the first unlinked occurrence of a name in one note. MatchedText
// and Location refer to the file text before NFC normalization.
type Mention struct {
	NodeInfo
	MatchedText string
	Location    LinkLocation
	Excerpt     string
	id          int64
}

// MentionsTotal counts filtered mentions before the limit. A direction that
// is not returned is nil.
type MentionsTotal struct {
	Incoming *int
	Outgoing *int
}

// MentionsResult keeps a direction that is not returned nil: Outgoing for a
// phantom entry and any direction not selected by Relations.
type MentionsResult struct {
	Entry    NodeInfo
	Incoming []Mention
	Outgoing []Mention
	Total    MentionsTotal
}

// Mentions reads the current note bodies to find unlinked occurrences of the
// entry name (incoming) and, for a note entry, of known note and phantom names
// in the entry body (outgoing). Linked nodes, the entry, and hidden notes are
// excluded in both directions.
func Mentions(vaultPath string, entry MentionsEntry, opts MentionsOptions, locations ...Locations) (*MentionsResult, error) {
	if (entry.File == "") == (entry.Phantom == "") {
		if entry.File == "" {
			return nil, errors.New("no entry specified: provide --file or --phantom")
		}
		return nil, errors.New("multiple entry specs: provide exactly one of --file, --phantom")
	}
	if opts.Limit != nil && *opts.Limit <= 0 {
		return nil, errors.New("limit must be positive")
	}
	if err := validateGlobPatterns(opts.Path); err != nil {
		return nil, err
	}
	wantIncoming, wantOutgoing, err := mentionDirections(opts.Relations, entry.File != "")
	if err != nil {
		return nil, err
	}
	sortKey, sortDesc, err := parseSortKey(opts.Sort)
	if err != nil {
		return nil, err
	}
	db, err := openDBChecked(vaultPath, locations...)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var entryID int64
	var info NodeInfo
	if entry.File != "" {
		entryID, info, err = findEntryByFile(db, entry.File)
		if err == nil && info.Type != NodeTypeNote {
			err = fmt.Errorf("mentions requires an indexed note: %s", entry.File)
		}
	} else {
		entryID, info, err = findEntryByPhantom(db, entry.Phantom)
	}
	if err != nil {
		return nil, err
	}
	linked, err := mentionLinkedIDs(db, entryID)
	if err != nil {
		return nil, err
	}
	candidates, err := mentionCandidates(db, entryID, linked, opts)
	if err != nil {
		return nil, err
	}
	var order *mentionOrder
	if sortKey != "" {
		if order, err = loadMentionOrder(db, sortKey, sortDesc); err != nil {
			return nil, err
		}
	}
	diskPaths := newReadVaultDiskPathResolver(vaultPath)
	result := &MentionsResult{Entry: info}

	if wantIncoming {
		needle := mentionFold(info.Name)
		var incoming []Mention
		for _, c := range candidates {
			if c.info.Type != NodeTypeNote {
				continue
			}
			content, ok, err := readMentionFile(diskPaths, c.info.Path)
			if err != nil {
				return nil, err
			}
			// Most notes do not contain the name at all; skip parsing them.
			if !ok || !strings.Contains(mentionFold(content), needle) {
				continue
			}
			if m, found := newMentionText(content).first(needle, opts.Partial); found {
				incoming = append(incoming, m.with(c))
			}
		}
		var total int
		result.Incoming, total = limitMentions(incoming, order, opts.Limit)
		result.Total.Incoming = &total
	}

	if wantOutgoing {
		content, ok, err := readMentionFile(diskPaths, info.Path)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("entry file not found: %s", info.Path)
		}
		text := newMentionText(content)
		byName := make(map[string][]queryNode)
		for _, c := range candidates {
			key := mentionFold(c.info.Name)
			byName[key] = append(byName[key], c)
		}
		outgoing := []Mention{}
		for key, nodes := range byName {
			m, found := text.first(key, opts.Partial)
			if !found {
				continue
			}
			for _, c := range nodes {
				outgoing = append(outgoing, m.with(c))
			}
		}
		var total int
		result.Outgoing, total = limitMentions(outgoing, order, opts.Limit)
		result.Total.Outgoing = &total
	}
	return result, nil
}

// mentionDirections validates the selected relations as query does. Outgoing
// is defined only for a note entry, so selecting it for a phantom is an error.
func mentionDirections(relations []string, fileEntry bool) (incoming, outgoing bool, err error) {
	if relations == nil {
		return true, fileEntry, nil
	}
	if len(relations) == 0 {
		return false, false, errors.New("relations: select at least one relation")
	}
	for _, r := range relations {
		var dir *bool
		switch r {
		case MentionsIncoming:
			dir = &incoming
		case MentionsOutgoing:
			dir = &outgoing
		default:
			return false, false, fmt.Errorf("relations: unknown relation %q", r)
		}
		if *dir {
			return false, false, fmt.Errorf("relations: duplicate relation %q", r)
		}
		*dir = true
	}
	if outgoing && !fileEntry {
		return false, false, errors.New("relations: outgoing requires a --file entry")
	}
	return incoming, outgoing, nil
}

// mentionLinkedIDs returns nodes linked with the entry in either direction.
func mentionLinkedIDs(db dbExecer, entryID int64) (map[int64]bool, error) {
	rows, err := db.Query(`SELECT source_id FROM edges WHERE target_id = ? UNION SELECT target_id FROM edges WHERE source_id = ?`, entryID, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	linked := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		linked[id] = true
	}
	return linked, rows.Err()
}

// mentionCandidates returns visible notes and phantoms other than the entry
// and its linked nodes, narrowed by the path, phantom, and where options.
func mentionCandidates(db dbExecer, entryID int64, linked map[int64]bool, opts MentionsOptions) ([]queryNode, error) {
	q := `SELECT id, type, name, COALESCE(path,''), exists_flag FROM nodes WHERE type IN ('note','phantom')`
	var args []any
	if opts.Where != nil {
		q += " AND type = 'note'"
		where, values := opts.Where.MetaFilterSQL("id")
		q += where
		args = values
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []queryNode
	for rows.Next() {
		id, info, err := scanNodeInfoWithID(rows)
		if err != nil {
			return nil, err
		}
		if id == entryID || linked[id] || opts.Filter.IsHidden(info) || mentionFold(info.Name) == "" {
			continue
		}
		if info.Type == NodeTypePhantom && opts.NoPhantom {
			continue
		}
		if info.Type == NodeTypeNote && !pathMatchesFilters(info.Path, opts.Path, nil) {
			continue
		}
		nodes = append(nodes, queryNode{id: id, info: info})
	}
	return nodes, rows.Err()
}

// readMentionFile reports ok=false when the indexed file no longer exists.
func readMentionFile(diskPaths *vaultDiskPathResolver, path string) (string, bool, error) {
	fullPath, err := diskPaths.existingPath(path)
	if err == nil {
		var content []byte
		content, err = os.ReadFile(fullPath)
		if err == nil {
			return string(content), true, nil
		}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	return "", false, err
}

// mentionOrder holds the search --sort value of each note that has the key.
// Computed fields compare as numbers and meta keys by compareSortValues.
type mentionOrder struct {
	numeric bool
	desc    bool
	nums    map[int64]int64
	strs    map[int64]string
}

// loadMentionOrder reads sort values as search does: computed fields for every
// note, and for a meta key the minimum value (maximum when descending).
func loadMentionOrder(db dbExecer, key string, desc bool) (*mentionOrder, error) {
	o := &mentionOrder{desc: desc}
	var q string
	var args []any
	if col, ok := computedSortColumn(key); ok {
		o.numeric = true
		o.nums = make(map[int64]int64)
		q = fmt.Sprintf("SELECT n.id, %s FROM nodes n WHERE n.type = 'note'", col)
	} else {
		o.strs = make(map[int64]string)
		agg := "MIN"
		if desc {
			agg = "MAX"
		}
		q = fmt.Sprintf("SELECT node_id, %s(sort_value) FROM meta WHERE key = ? GROUP BY node_id", agg)
		args = []any{key}
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if o.numeric {
			var n int64
			if err := rows.Scan(&id, &n); err != nil {
				return nil, err
			}
			o.nums[id] = n
			continue
		}
		var v sql.NullString
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		if v.Valid {
			o.strs[id] = v.String
		}
	}
	return o, rows.Err()
}

// compare orders mentions with the key before those without it; ok is false
// when both lack the key or their values are equal.
func (o *mentionOrder) compare(a, b int64) (less, ok bool) {
	var c int
	if o.numeric {
		x, xok := o.nums[a]
		y, yok := o.nums[b]
		if xok != yok {
			return xok, true
		}
		c = cmp.Compare(x, y)
	} else {
		x, xok := o.strs[a]
		y, yok := o.strs[b]
		if xok != yok {
			return xok, true
		}
		c = compareSortValues(x, y)
	}
	if c == 0 {
		return false, false
	}
	return (c < 0) != o.desc, true
}

// limitMentions sorts by order (nil for the default order), then by the
// default order, and keeps at most limit mentions.
func limitMentions(mentions []Mention, order *mentionOrder, limit *int) ([]Mention, int) {
	if mentions == nil {
		mentions = []Mention{}
	}
	sort.Slice(mentions, func(i, j int) bool {
		if order != nil {
			if less, ok := order.compare(mentions[i].id, mentions[j].id); ok {
				return less
			}
		}
		return queryNodeLess(queryNode{mentions[i].id, mentions[i].NodeInfo}, queryNode{mentions[j].id, mentions[j].NodeInfo})
	})
	total := len(mentions)
	if limit != nil && *limit < total {
		mentions = mentions[:*limit]
	}
	return mentions, total
}

func mentionFold(s string) string {
	return strings.ToLower(normalizeTextNFC(s))
}

// mentionSegment is one NFC normalization segment of the file text. A barrier
// segment covers excluded text and never takes part in a match.
type mentionSegment struct {
	rawStart, rawEnd   int
	foldStart, foldEnd int
	starter            rune
}

// mentionText is the searchable body of one file. folded is the NFC lower-case
// form of the body with excluded text replaced by NUL barriers; segment
// offsets map it back to the original bytes.
type mentionText struct {
	content  string
	folded   string
	segments []mentionSegment
	startAt  map[int]int
	endAt    map[int]int
}

// mentionMatch is a matched range of the original content.
type mentionMatch struct {
	text     string
	location LinkLocation
	excerpt  string
}

func (m mentionMatch) with(node queryNode) Mention {
	return Mention{NodeInfo: node.info, MatchedText: m.text, Location: m.location, Excerpt: m.excerpt, id: node.id}
}

func newMentionText(content string) *mentionText {
	t := &mentionText{content: content, startAt: make(map[int]int), endAt: make(map[int]int)}
	lines := strings.Split(content, "\n")
	lineOffsets := make([]int, len(lines))
	for i, offset := 1, 0; i < len(lines); i++ {
		offset += len(lines[i-1]) + 1
		lineOffsets[i] = offset
	}
	fmEnd := frontmatterEnd(lines)
	definitions := bodyReferenceDefinitions(lines, fmEnd)
	var folded strings.Builder
	walkBodyParagraphs(lines, fmEnd, func(lineNum int, raw, clean string) {
		_, masked := parseBodyMarkdown(raw, maskParagraphWikiLinks(clean), lineNum, definitions)
		barrier := mentionBarriers(raw, maskURLs(masked))
		base := lineOffsets[lineNum-1]
		for i := 0; i < len(raw); {
			end := i + norm.NFC.NextBoundaryInString(raw[i:], true)
			seg := mentionSegment{rawStart: base + i, rawEnd: base + end, foldStart: folded.Len()}
			seg.starter, _ = utf8.DecodeRuneInString(raw[i:end])
			blocked := false
			for j := i; j < end; j++ {
				blocked = blocked || barrier[j]
			}
			if blocked {
				folded.WriteByte(0)
				seg.starter = 0
			} else {
				folded.WriteString(mentionFold(raw[i:end]))
			}
			seg.foldEnd = folded.Len()
			t.startAt[seg.foldStart] = len(t.segments)
			t.endAt[seg.foldEnd] = len(t.segments)
			t.segments = append(t.segments, seg)
			i = end
		}
		// Separate paragraphs so a name never spans two of them.
		t.segments = append(t.segments, mentionSegment{rawStart: base + len(raw), rawEnd: base + len(raw), foldStart: folded.Len(), foldEnd: folded.Len() + 1})
		folded.WriteByte(0)
	})
	t.folded = folded.String()
	return t
}

// first returns the earliest occurrence of needle, an already folded name,
// whose range aligns with whole segments and satisfies the letter boundary.
func (t *mentionText) first(needle string, partial bool) (mentionMatch, bool) {
	if needle == "" {
		return mentionMatch{}, false
	}
	for from := 0; ; {
		i := strings.Index(t.folded[from:], needle)
		if i < 0 {
			return mentionMatch{}, false
		}
		start := from + i
		from = start + 1
		first, ok := t.startAt[start]
		if !ok {
			continue
		}
		last, ok := t.endAt[start+len(needle)]
		if !ok {
			continue
		}
		if !partial && (t.latinJoin(first-1, first) || t.latinJoin(last, last+1)) {
			continue
		}
		return t.match(t.segments[first].rawStart, t.segments[last].rawEnd), true
	}
}

// latinJoin reports whether two adjacent segments both start with a Latin
// letter, which places a match boundary in the middle of a Latin word.
func (t *mentionText) latinJoin(a, b int) bool {
	if a < 0 || b >= len(t.segments) {
		return false
	}
	return isLatinLetter(t.segments[a].starter) && isLatinLetter(t.segments[b].starter)
}

func isLatinLetter(r rune) bool {
	return unicode.Is(unicode.Latin, r)
}

func (t *mentionText) match(start, end int) mentionMatch {
	lineStart := strings.LastIndexByte(t.content[:start], '\n') + 1
	lineEnd := strings.IndexByte(t.content[start:], '\n')
	if lineEnd < 0 {
		lineEnd = len(t.content)
	} else {
		lineEnd += start
	}
	return mentionMatch{
		text:     t.content[start:end],
		location: LinkLocation{Start: contentPosition(t.content, start), End: contentPosition(t.content, end)},
		excerpt:  strings.TrimSuffix(t.content[lineStart:lineEnd], "\r"),
	}
}

func contentPosition(content string, offset int) LinkPosition {
	lineStart := strings.LastIndexByte(content[:offset], '\n') + 1
	return LinkPosition{
		Line:   strings.Count(content[:offset], "\n") + 1,
		Column: utf8.RuneCountInString(content[lineStart:offset]) + 1,
	}
}

// mentionBarriers marks bytes blanked by masking, extended over adjacent
// spaces and tabs, so masked spaces cannot match spaces inside a name.
func mentionBarriers(raw, masked string) []bool {
	barrier := make([]bool, len(raw))
	for i := range barrier {
		barrier[i] = i >= len(masked) || masked[i] != raw[i]
	}
	blank := func(i int) bool { return raw[i] == ' ' || raw[i] == '\t' }
	for i := range barrier {
		if !barrier[i] {
			continue
		}
		for j := i - 1; j >= 0 && !barrier[j] && blank(j); j-- {
			barrier[j] = true
		}
		for j := i + 1; j < len(raw) && !barrier[j] && blank(j); j++ {
			barrier[j] = true
		}
	}
	return barrier
}

// maskURLs blanks text from a "scheme://" or "mailto:" prefix to the next
// whitespace, keeping byte offsets and line breaks.
func maskURLs(text string) string {
	masked := []byte(text)
	blankUntilSpace := func(start int) int {
		end := start
		for end < len(text) {
			r, size := utf8.DecodeRuneInString(text[end:])
			if unicode.IsSpace(r) {
				break
			}
			end += size
		}
		for i := start; i < end; i++ {
			masked[i] = ' '
		}
		return end
	}
	for i := 0; i < len(text); {
		// Compare in place: lower-casing may change byte lengths before i.
		if len(text)-i >= len("mailto:") && strings.EqualFold(text[i:i+len("mailto:")], "mailto:") {
			i = blankUntilSpace(i)
			continue
		}
		if strings.HasPrefix(text[i:], "://") {
			start := i
			for start > 0 && isURISchemeByte(text[start-1]) {
				start--
			}
			for start < i && !isASCIILetter(text[start]) {
				start++
			}
			if start < i && isURIScheme(text[start:i]) {
				i = blankUntilSpace(start)
				continue
			}
		}
		i++
	}
	return string(masked)
}

func isURISchemeByte(b byte) bool {
	return isASCIILetter(b) || (b >= '0' && b <= '9') || b == '+' || b == '-' || b == '.'
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
