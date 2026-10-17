package main

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/cplieger/langtag/v3/internal/multiscript"
)

// writeDiff writes to w a Markdown report of how the script table moves from
// the rows old to next: the languages entering and leaving the MultiScript
// set, then every row added, removed or given another script.
// It writes nothing when the tables hold the same rows.
func writeDiff(w io.Writer, old, next []row) error {
	changes := changedRows(old, next)
	if len(changes) == 0 {
		return nil
	}
	before, after := multiScript(old), multiScript(next)
	var b strings.Builder
	b.WriteString("### Script table: MultiScript languages\n\n")
	fmt.Fprintf(&b, "Count: %d before, %d after.\n\n", len(before), len(after))
	fmt.Fprintf(&b, "- Entering: %s\n", codeList(without(after, before)))
	fmt.Fprintf(&b, "- Leaving: %s\n", codeList(without(before, after)))
	fmt.Fprintf(&b, "\n### Script table: changed rows (%d)\n\n", len(changes))
	b.WriteString("| Row | Before | After |\n|---|---|---|\n")
	for _, c := range changes {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", c.key, code(c.before), code(c.after))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// multiScript returns the languages Tag.MultiScript answers true for under
// rows, those reached through their macrolanguage included.
func multiScript(rows []row) []string {
	var keyed []string
	for _, r := range rows {
		if r.region != "" {
			keyed = append(keyed, r.language)
		}
	}
	slices.Sort(keyed)
	keyed = slices.Compact(keyed)
	return multiscript.Languages(func(l string) bool {
		_, ok := slices.BinarySearch(keyed, l)
		return ok
	})
}

// rowChange is one key whose script differs between the tables; "" marks
// the side that lacks the row.
type rowChange struct{ key, before, after string }

// changedRows returns every key whose script differs between old and next,
// sorted by key.
func changedRows(old, next []row) []rowChange {
	scripts := make(map[string]*rowChange, len(old))
	for _, r := range old {
		scripts[rowKey(r)] = &rowChange{key: rowKey(r), before: r.script}
	}
	for _, r := range next {
		k := rowKey(r)
		if c, ok := scripts[k]; ok {
			c.after = r.script
			continue
		}
		scripts[k] = &rowChange{key: k, after: r.script}
	}
	var out []rowChange
	for _, c := range scripts {
		if c.before != c.after {
			out = append(out, *c)
		}
	}
	slices.SortFunc(out, func(a, b rowChange) int { return strings.Compare(a.key, b.key) })
	return out
}

func rowKey(r row) string {
	if r.region == "" {
		return r.language
	}
	return r.language + "_" + r.region
}

// without returns the members of a, sorted, that b lacks.
func without(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) {
			out = append(out, s)
		}
	}
	return out
}

func codeList(langs []string) string {
	if len(langs) == 0 {
		return "none"
	}
	quoted := make([]string, len(langs))
	for i, l := range langs {
		quoted[i] = code(l)
	}
	return strings.Join(quoted, ", ")
}

func code(s string) string {
	if s == "" {
		return "none"
	}
	return "`" + s + "`"
}
