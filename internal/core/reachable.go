package core

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
)

// ReachableOptions controls the reachability check.
type ReachableOptions struct {
	From string // vault-relative path of the entry note (required)
	// Path / Exclude filter the target note set by path glob. Empty Path
	// means all existing notes.
	Path    []string
	Exclude []string
	Route   bool // compute shortest routes for reachable target notes
}

// ReachableResult partitions the target note set by reachability from the
// entry note. The entry itself is reachable (0 hops) when it is in the set;
// notes outside the set appear in neither list.
type ReachableResult struct {
	From        string              // normalized entry path
	Reachable   []string            // sorted target note paths reachable from the entry
	Unreachable []string            // sorted target note paths not reachable
	Routes      map[string][]string // target path → shortest route (entry .. target); set when Route
}

// traversalLinkTypeSQLList enumerates the link types the reachability walk
// follows: note-to-note navigation links. Tag edges (tag, frontmatter) are
// excluded so sharing a tag never counts as reachable.
const traversalLinkTypeSQLList = `'wikilink', 'markdown', 'frontmatter_wikilink', 'frontmatter_path'`

// Reachable walks outgoing links from the entry note (BFS over the edges
// table) and partitions the target note set into reachable / unreachable.
func Reachable(vaultPath string, opts ReachableOptions) (*ReachableResult, error) {
	if opts.From == "" {
		return nil, errors.New("no entry specified: provide --from")
	}
	if err := validateGlobPatterns(opts.Path); err != nil {
		return nil, err
	}
	if err := validateGlobPatterns(opts.Exclude); err != nil {
		return nil, err
	}

	db, err := openDBChecked(vaultPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	from := NormalizePath(opts.From)
	var fromID int64
	err = db.QueryRow(
		`SELECT id FROM nodes WHERE node_key = ? AND type='note' AND exists_flag=1`,
		noteKey(from)).Scan(&fromID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s (reachable requires an existing note as --from)", ErrFileNotRegistered, from)
	}
	if err != nil {
		return nil, err
	}

	// Target note set. type='note' AND exists_flag=1 guarantees a non-NULL
	// path, so the GLOB filters never hit NULL three-valued logic.
	inclSQL, inclArgs := pathIncludeSQL("path", opts.Path)
	ef := &ExcludeFilter{PathGlobs: opts.Exclude}
	exclSQL, exclArgs := ef.PathExcludeSQL("path")
	targets, err := queryIDPathMap(db,
		`SELECT id, path FROM nodes WHERE type='note' AND exists_flag=1`+inclSQL+exclSQL,
		append(inclArgs, exclArgs...)...)
	if err != nil {
		return nil, err
	}

	// Adjacency over navigation edges from existing notes. Asset and phantom
	// targets enter the visited set but have no outgoing entries, so they
	// act as leaves.
	adj := make(map[int64][]int64)
	rows, err := db.Query(fmt.Sprintf(
		`SELECT e.source_id, e.target_id FROM edges e
		 JOIN nodes sn ON sn.id = e.source_id AND sn.type='note' AND sn.exists_flag=1
		 WHERE e.link_type IN (%s)`, traversalLinkTypeSQLList))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var src, tgt int64
		if err := rows.Scan(&src, &tgt); err != nil {
			return nil, err
		}
		adj[src] = append(adj[src], tgt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// BFS from the entry; parent records the shortest-path tree for Routes.
	visited := map[int64]bool{fromID: true}
	parent := make(map[int64]int64)
	queue := []int64{fromID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if visited[next] {
				continue
			}
			visited[next] = true
			parent[next] = cur
			queue = append(queue, next)
		}
	}

	result := &ReachableResult{From: from}
	for id, p := range targets {
		if visited[id] {
			result.Reachable = append(result.Reachable, p)
		} else {
			result.Unreachable = append(result.Unreachable, p)
		}
	}
	sort.Strings(result.Reachable)
	sort.Strings(result.Unreachable)

	if opts.Route {
		// Route nodes may lie outside the target set (connector notes), so
		// render routes from the full note path map. Without filters the
		// target set already is that map.
		notePaths := targets
		if len(opts.Path) > 0 || len(opts.Exclude) > 0 {
			notePaths, err = queryIDPathMap(db,
				`SELECT id, path FROM nodes WHERE type='note' AND exists_flag=1`)
			if err != nil {
				return nil, err
			}
		}
		result.Routes = make(map[string][]string, len(result.Reachable))
		for id, p := range targets {
			if !visited[id] {
				continue
			}
			// The parent chain holds only existing notes: adj sources are
			// restricted to type='note' AND exists_flag=1, so every hop is
			// covered by notePaths. Keep that JOIN in sync with this lookup.
			var route []string
			for cur := id; ; cur = parent[cur] {
				route = append(route, notePaths[cur])
				if cur == fromID {
					break
				}
			}
			slices.Reverse(route)
			result.Routes[p] = route
		}
	}

	return result, nil
}

// queryIDPathMap runs a query returning (id, path) rows into a map.
func queryIDPathMap(db dbExecer, query string, args ...any) (map[int64]string, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[int64]string)
	for rows.Next() {
		var id int64
		var p string
		if err := rows.Scan(&id, &p); err != nil {
			return nil, err
		}
		m[id] = p
	}
	return m, rows.Err()
}
