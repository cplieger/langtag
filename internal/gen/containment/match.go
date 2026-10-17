package main

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/cplieger/langtag/v3/internal/gen/subtag"
)

// side is one side of a languageMatch rule rating a region difference: a
// language and a script, each a subtag or "*", and a region, which is a
// code, "*", a variable $v or its complement $!v.
type side struct{ language, script, region string }

// regionRule is one languageMatch rule whose sides have three fields.
type regionRule struct {
	desired, supported side
	text               string
	distance           int
	oneway             bool
}

// parseRegionRules returns, in file order, the rules that rate a region
// difference. It refuses a variable outside a region field, an undeclared
// variable, a malformed field and a missing or malformed distance or oneway.
func parseRegionRules(rules []languageMatch, variables map[string][]string) ([]regionRule, error) {
	var out []regionRule
	for _, m := range rules {
		text := m.Desired + " => " + m.Supported
		desired, dok, err := parseSide(m.Desired, variables)
		if err != nil {
			return nil, fmt.Errorf("languageMatch %s: %w", text, err)
		}
		supported, sok, err := parseSide(m.Supported, variables)
		if err != nil {
			return nil, fmt.Errorf("languageMatch %s: %w", text, err)
		}
		if dok != sok {
			return nil, fmt.Errorf("languageMatch %s: want both sides <language>_<script>_<region> or neither", text)
		}
		if !dok {
			continue
		}
		d, err := strconv.Atoi(m.Distance)
		if err != nil || d < 0 {
			return nil, fmt.Errorf("languageMatch %s: distance %q, want a non-negative integer", text, m.Distance)
		}
		if m.Oneway != "" && m.Oneway != "true" && m.Oneway != "false" {
			return nil, fmt.Errorf("languageMatch %s: oneway %q, want true or false", text, m.Oneway)
		}
		out = append(out, regionRule{desired: desired, supported: supported, distance: d, oneway: m.Oneway == "true", text: text})
	}
	return out, nil
}

// parseSide returns s as a side when it has three fields. CLDR allows a
// variable only as the region field.
func parseSide(s string, variables map[string][]string) (side, bool, error) {
	f := strings.Split(s, "_")
	if len(f) != 3 {
		if strings.Contains(s, "$") {
			return side{}, false, fmt.Errorf("%s: want a variable only as the region of <language>_<script>_<region>", s)
		}
		return side{}, false, nil
	}
	if f[0] != "*" && !subtag.Language.MatchString(f[0]) {
		return side{}, false, fmt.Errorf("%s: language %q, want * or a language subtag", s, f[0])
	}
	if f[1] != "*" && !subtag.Script.MatchString(f[1]) {
		return side{}, false, fmt.Errorf("%s: script %q, want * or a script subtag", s, f[1])
	}
	region := f[2]
	if v, ok := strings.CutPrefix(region, "$"); ok {
		if _, declared := variables["$"+strings.TrimPrefix(v, "!")]; !declared {
			return side{}, false, fmt.Errorf("%s: no matchVariable declares %s", s, region)
		}
	} else if region != "*" && !subtag.Region.MatchString(region) {
		return side{}, false, fmt.Errorf("%s: region %q, want *, a variable or a region subtag", s, region)
	}
	return side{language: f[0], script: f[1], region: region}, true, nil
}

// matcher rates region differences as UTS #35's language matching does
// (https://www.unicode.org/reports/tr35/tr35.html#EnhancedLanguageMatching):
// the rules are ordered and the first one matching a pair gives its
// distance. A variable holds the regions resolve gives it, and a region
// code the regions it equals or contains.
type matcher struct {
	variables, ancestors map[string][]string
	rules                []regionRule
}

// distance returns the distance the first rule matching desired =>
// supported gives two tags of language and script, or false when none
// does. A rule not marked oneway also matches supported => desired.
func (m *matcher) distance(language, script, desired, supported string) (int, bool) {
	for i := range m.rules {
		r := &m.rules[i]
		if (m.matches(r.desired, language, script, desired) && m.matches(r.supported, language, script, supported)) ||
			(!r.oneway && m.matches(r.desired, language, script, supported) && m.matches(r.supported, language, script, desired)) {
			return r.distance, true
		}
	}
	return 0, false
}

func (m *matcher) matches(s side, language, script, region string) bool {
	return (s.language == "*" || s.language == language) && (s.script == "*" || s.script == script) && m.inRegion(s.region, region)
}

func (m *matcher) inRegion(pattern, region string) bool {
	switch {
	case pattern == "*":
		return true
	case strings.HasPrefix(pattern, "$!"):
		return !m.inVariable("$"+pattern[2:], region)
	case strings.HasPrefix(pattern, "$"):
		return m.inVariable(pattern, region)
	default:
		return m.contains(pattern, region)
	}
}

func (m *matcher) inVariable(variable, region string) bool {
	_, found := slices.BinarySearch(m.variables[variable], region)
	return found
}

func (m *matcher) contains(parent, child string) bool { return within(m.ancestors, parent, child) }

// within reports whether parent is child or holds it in ancestors.
func within(ancestors map[string][]string, parent, child string) bool {
	return parent == child || slices.Contains(ancestors[child], parent)
}

// cluster is one regionClusters entry: a language, the script the rules
// limit it to or "", and its variable.
type cluster struct{ language, script, variable string }

// evaluateClusters returns, sorted, a cluster for every language and script
// a region rule names a variable for, each holding only when split finds
// the evaluated distances divide the regions at the variable, and refuses a
// declared variable no rule names, which regionClusters could not bind.
func evaluateClusters(rules []languageMatch, variables, ancestors map[string][]string) ([]cluster, error) {
	parsed, err := parseRegionRules(rules, variables)
	if err != nil {
		return nil, err
	}
	m := &matcher{rules: parsed, variables: variables, ancestors: ancestors}
	out, err := m.named()
	if err != nil {
		return nil, err
	}
	regions := placed(ancestors)
	for _, c := range out {
		for _, script := range m.scripts(c) {
			if err := m.split(c, script, regions); err != nil {
				return nil, err
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(variables)) {
		if !slices.ContainsFunc(out, func(c cluster) bool { return c.variable == id }) {
			return nil, fmt.Errorf("matchVariable %s: no languageMatch rule names it for a language", id)
		}
	}
	return out, nil
}

// named returns, sorted, the clusters the rules name a variable for,
// refusing what regionClusters cannot hold: a variable named for every
// language and two clusters of one language whose scripts overlap.
func (m *matcher) named() ([]cluster, error) {
	named := map[cluster]bool{}
	for i := range m.rules {
		r := &m.rules[i]
		for _, s := range []side{r.desired, r.supported} {
			v, ok := strings.CutPrefix(s.region, "$")
			if !ok {
				continue
			}
			v = "$" + strings.TrimPrefix(v, "!")
			if s.language == "*" {
				return nil, fmt.Errorf("languageMatch %s names %s for every language, want one language per cluster", r.text, v)
			}
			named[cluster{language: s.language, script: strings.TrimPrefix(s.script, "*"), variable: v}] = true
		}
	}
	out := slices.SortedFunc(maps.Keys(named), func(x, y cluster) int {
		return cmp.Or(strings.Compare(x.language, y.language), strings.Compare(x.script, y.script), strings.Compare(x.variable, y.variable))
	})
	return out, overlap(out)
}

// overlap refuses two of clusters for one language whose scripts overlap.
func overlap(clusters []cluster) error {
	for i, c := range clusters {
		for _, o := range clusters[:i] {
			if o.language == c.language && (o.script == c.script || o.script == "" || c.script == "") {
				return fmt.Errorf("languageMatch rules name %s and %s for %s, two clusters for one language and script", o.variable, c.variable, c.language)
			}
		}
	}
	return nil
}

// placed returns, sorted, every region ancestors places: each contained
// region and each group.
func placed(ancestors map[string][]string) []string {
	set := map[string]bool{}
	for r, groups := range ancestors {
		set[r] = true
		for _, g := range groups {
			set[g] = true
		}
	}
	return slices.Sorted(maps.Keys(set))
}

// scripts returns the scripts c is evaluated in: its own, or for a cluster
// of every script each script a rule for its language names, and "", which
// stands for every script no rule names.
func (m *matcher) scripts(c cluster) []string {
	if c.script != "" {
		return []string{c.script}
	}
	out := []string{""}
	for i := range m.rules {
		r := &m.rules[i]
		for _, s := range []side{r.desired, r.supported} {
			if s.script != "*" && (s.language == "*" || s.language == c.language) && !slices.Contains(out, s.script) {
				out = append(out, s.script)
			}
		}
	}
	slices.Sort(out)
	return out
}

// pair is two regions and the distance the rules give them.
type pair struct {
	desired, supported string
	distance           int
}

// split checks that, for tags of c's language in script, the rules rate
// every pair of regions on one side of c's variable closer than every pair
// across it. It reads the pairs the tier grades: distinct regions, neither
// containing the other.
func (m *matcher) split(c cluster, script string, regions []string) error {
	near, far, err := m.extremes(c, script, regions)
	if err != nil {
		return err
	}
	if near == nil || far == nil {
		return fmt.Errorf("%s for %s: no pair of regions on one side or none across it", c.variable, spell(c.language, script, ""))
	}
	if near.distance >= far.distance {
		return fmt.Errorf("languageMatch rules do not split %s at %s: %s, on one side, rates %d, and %s, across it, rates %d",
			spell(c.language, script, ""), c.variable, near.spell(c.language, script), near.distance, far.spell(c.language, script), far.distance)
	}
	return nil
}

// extremes returns the first pair of regions on one side of c's variable
// rated farthest apart and the first pair across it rated nearest, each nil
// when there is none, refusing a pair no rule rates.
func (m *matcher) extremes(c cluster, script string, regions []string) (near, far *pair, err error) {
	for _, a := range regions {
		for _, b := range regions {
			if m.contains(a, b) || m.contains(b, a) {
				continue
			}
			d, ok := m.distance(c.language, script, a, b)
			if !ok {
				return nil, nil, fmt.Errorf("no languageMatch rule rates %s against %s", spell(c.language, script, a), spell(c.language, script, b))
			}
			p := &pair{desired: a, supported: b, distance: d}
			if m.inVariable(c.variable, a) != m.inVariable(c.variable, b) {
				far = closer(far, p)
			} else {
				near = farther(near, p)
			}
		}
	}
	return near, far, nil
}

// closer returns the nearer of best and p, best when they tie.
func closer(best, p *pair) *pair {
	if best == nil || p.distance < best.distance {
		return p
	}
	return best
}

// farther returns the farther of best and p, best when they tie.
func farther(best, p *pair) *pair {
	if best == nil || p.distance > best.distance {
		return p
	}
	return best
}

// spell writes p as two tags of language and script: es-FR => es-ES.
func (p *pair) spell(language, script string) string {
	return spell(language, script, p.desired) + " => " + spell(language, script, p.supported)
}

// spell writes a tag of language, script and region, leaving out an empty
// subtag: zh-Hant-HK, es-MX, es.
func spell(language, script, region string) string {
	return strings.Join(slices.DeleteFunc([]string{language, script, region}, func(s string) bool { return s == "" }), "-")
}
