package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/langtag/v3/internal/gen/cldr"
)

func doc(groups string) []byte {
	return []byte("<supplementalData><territoryContainment>" + groups + "</territoryContainment></supplementalData>")
}

func info(body string) []byte {
	return []byte(`<supplementalData><languageMatching><languageMatches type="written_new">` + body +
		`</languageMatches></languageMatching></supplementalData>`)
}

// Rows shaped like release 48's territoryContainment: a partition row and a
// grouping row for one group, a grouping="true" group, and a deprecated row.
const fixtureGroups = `<group type="001" contains="019 009"/>` +
	`<group type="019" contains="021 013 005"/>` +
	`<group type="019" contains="419" status="grouping"/>` +
	`<group type="419" contains="013 005" grouping="true"/>` +
	`<group type="013" contains="MX PA"/>` +
	`<group type="013" contains="PZ" status="deprecated"/>` +
	`<group type="021" contains="US"/>` +
	`<group type="009" contains="QO"/>` +
	`<group type="QO" contains="AQ"/>`

// worldGroups are release-48 containment rows reaching a few members of
// each cluster and regions outside each, with the EU grouping across them.
const worldGroups = `<group type="001" contains="019 002 150 142 009"/>` +
	`<group type="019" contains="021 013 029 005"/>` +
	`<group type="019" contains="419" status="grouping"/>` +
	`<group type="419" contains="013 029 005" grouping="true"/>` +
	`<group type="021" contains="US CA"/>` +
	`<group type="013" contains="MX PA"/>` +
	`<group type="029" contains="PR"/>` +
	`<group type="005" contains="AR BR"/>` +
	`<group type="002" contains="015"/>` +
	`<group type="015" contains="MA DZ EG"/>` +
	`<group type="150" contains="154 155 039"/>` +
	`<group type="154" contains="GB IE"/>` +
	`<group type="155" contains="FR DE"/>` +
	`<group type="039" contains="ES PT IT"/>` +
	`<group type="142" contains="030 035"/>` +
	`<group type="030" contains="CN HK MO TW"/>` +
	`<group type="035" contains="PH"/>` +
	`<group type="009" contains="053 057"/>` +
	`<group type="053" contains="AU NZ"/>` +
	`<group type="057" contains="GU MH"/>` +
	`<group type="EU" contains="DE ES FR IE IT PT" status="grouping"/>`

// variables48 are languageInfo.xml's matchVariable entries at release 48
// (lines 13-16).
const variables48 = `<matchVariable id="$enUS" value="AS+CA+GU+MH+MP+PH+PR+UM+US+VI"/>` +
	`<matchVariable id="$cnsar" value="HK+MO"/>` +
	`<matchVariable id="$americas" value="019"/>` +
	`<matchVariable id="$maghreb" value="MA+DZ+TN+LY+MR+EH"/>`

func match(desired, supported string, d int) string {
	return fmt.Sprintf(`<languageMatch desired="%s" supported="%s" distance="%d"/>`, desired, supported, d)
}

func self(s string, d int) string { return match(s, s, d) }

// family is the three rules release 48 rates a cluster with: its variable's
// and its complement's self matches at inside, then scope's wildcard region
// at across.
func family(scope, variable string, inside, across int) string {
	return self(scope+"$"+variable, inside) + self(scope+"$!"+variable, inside) + self(scope+"*", across)
}

var (
	ar48 = family("ar_*_", "maghreb", 4, 5)
	en48 = self("en_*_$enUS", 4) + match("en_*_$!enUS", "en_*_GB", 3) + self("en_*_$!enUS", 4) + self("en_*_*", 5)
	es48 = family("es_*_", "americas", 4, 5)
	pt48 = family("pt_*_", "americas", 4, 5)
	zh48 = family("zh_Hant_", "cnsar", 4, 5)
	// default48 is the generic region rule every list ends with (line 456).
	default48 = self("*_*_*", 4)
	// other48 are rules of other levels, which rate no region difference.
	other48 = match("nn", "nb", 4) + `<languageMatch desired="zh_Hant" supported="zh_Hans" distance="19" oneway="true"/>`
	// rules48 are release 48's written_new rules rating regions (lines
	// 440-456), after rules of other levels.
	rules48 = other48 + ar48 + en48 + es48 + pt48 + zh48 + default48
)

const clusters48 = `{language: "ar", variable: "$maghreb"}, {language: "en", variable: "$enUS"}, ` +
	`{language: "es", variable: "$americas"}, {language: "pt", variable: "$americas"}, ` +
	`{language: "zh", script: "Hant", variable: "$cnsar"},`

// customVariables use the rest of UTS #35's matchVariable grammar: a digit
// id, a numeric macroregion, a set difference read left to right (019-021-US
// is (019-021)-US, which holds no US, not 019-(021-US), which does) and a
// union after a difference.
const customVariables = `<matchVariable id="$1" value="019-021-US"/>` +
	`<matchVariable id="$x2" value="019-013+MX"/>`

// TestUTS35FirstMatch checks the matcher against distances worked out by
// hand from release 48's rules: the first matching rule rates a pair, in
// either direction unless it is oneway, and a variable or a region code
// holds what it contains.
func TestUTS35FirstMatch(t *testing.T) {
	t.Parallel()
	custom := `<languageMatch desired="xx_*_FR" supported="xx_*_DE" distance="7" oneway="true"/>` +
		match("xx_*_FR", "xx_*_*", 6) + self("yy_*_150", 2) +
		family("ww_*_", "1", 4, 5) + family("zz_*_", "x2", 4, 5)
	m48, mCustom := newMatcher(t, variables48, rules48), newMatcher(t, variables48+customVariables, custom)
	for _, c := range []struct {
		m                                    *matcher
		language, script, desired, supported string
		want                                 int
		found                                bool
	}{
		{m48, "en", "", "US", "GB", 5, true},     // across $enUS: en_*_*
		{m48, "en", "", "GB", "US", 5, true},     // the same rules the other way
		{m48, "en", "", "IE", "GB", 3, true},     // en_*_$!enUS => en_*_GB
		{m48, "en", "", "GB", "IE", 3, true},     // that rule reversed, not being oneway
		{m48, "en", "", "IE", "AU", 4, true},     // en_*_$!enUS self match
		{m48, "en", "", "US", "PR", 4, true},     // en_*_$enUS self match
		{m48, "es", "", "MX", "ES", 5, true},     // across $americas
		{m48, "es", "", "MX", "AR", 4, true},     // inside $americas, through 019's members
		{m48, "es", "", "419", "MX", 4, true},    // 019 contains 419 too
		{m48, "es", "", "ES", "FR", 4, true},     // es_*_$!americas
		{m48, "zh", "Hant", "HK", "TW", 5, true}, // across $cnsar
		{m48, "zh", "Hant", "HK", "MO", 4, true}, // inside $cnsar
		{m48, "zh", "Hans", "HK", "TW", 4, true}, // zh_Hant rules skip Hans: *_*_*
		{m48, "fr", "", "FR", "CA", 4, true},     // a language with no cluster: *_*_*
		{m48, "ar", "", "MA", "EG", 5, true},     // across $maghreb
		{m48, "ar", "", "MA", "DZ", 4, true},     // inside $maghreb
		{mCustom, "xx", "", "FR", "DE", 7, true}, // the oneway rule, its own direction
		{mCustom, "xx", "", "DE", "FR", 6, true}, // past it, reversed xx_*_FR => xx_*_*
		{mCustom, "xx", "", "DE", "IT", 0, false},
		{mCustom, "yy", "", "FR", "DE", 2, true}, // a region code holds what it contains
		{mCustom, "yy", "", "FR", "MX", 0, false},
		{mCustom, "ww", "", "MX", "AR", 4, true},  // inside $1, through 019's members
		{mCustom, "ww", "", "US", "AR", 5, true},  // 021, then US, subtracted
		{mCustom, "ww", "", "US", "CA", 4, true},  // both subtracted, so outside
		{mCustom, "ww", "", "021", "AR", 5, true}, // a group with no member left is outside
		{mCustom, "ww", "", "019", "ES", 4, true}, // 019 holds US, outside $1, so it is outside
		{mCustom, "zz", "", "MX", "AR", 4, true},  // MX added back after 013
		{mCustom, "zz", "", "PA", "AR", 5, true},  // PA left out with 013
		{mCustom, "zz", "", "013", "ES", 4, true}, // 013 straddles $x2, so it is outside
	} {
		got, found := c.m.distance(c.language, c.script, c.desired, c.supported)
		if got != c.want || found != c.found {
			t.Errorf("distance(%s) = %d, %v; want %d, %v",
				spell(c.language, c.script, c.desired)+" => "+spell(c.language, c.script, c.supported), got, found, c.want, c.found)
		}
	}
}

// newMatcher returns the matcher of variables and rules over worldGroups.
func newMatcher(t *testing.T, variables, rules string) *matcher {
	t.Helper()
	var d supplementalData
	if err := xml.Unmarshal(doc(worldGroups), &d); err != nil {
		t.Fatal(err)
	}
	table, err := ancestors(d.Groups)
	if err != nil {
		t.Fatal(err)
	}
	block, err := writtenNew(info(variables + rules))
	if err != nil {
		t.Fatal(err)
	}
	held, err := parseVariables(block.Variables, table)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseRegionRules(block.Rules, held)
	if err != nil {
		t.Fatal(err)
	}
	return &matcher{rules: parsed, variables: held, ancestors: table}
}

// TestClusterSplitEvaluated checks the clusters come from evaluating the
// rules: release 48's give its five, a change keeping every pair across a
// variable farther apart than every pair on one side keeps the cluster, and
// a change breaking that is refused naming the pairs.
func TestClusterSplitEvaluated(t *testing.T) {
	t.Parallel()
	es := `{language: "es", variable: "$americas"},`
	frJoins := strings.Replace(rules48, default48, family("fr_*_", "1", 4, 5)+default48, 1)
	accepted := map[string]struct{ rules, want string }{
		"release 48": {rules48, clusters48},
		"families reordered": {
			other48 + zh48 + pt48 + self("es_*_$!americas", 4) + self("es_*_$americas", 4) + self("es_*_*", 5) + en48 + ar48 + default48,
			clusters48,
		},
		// es-ES rated closer to its own side, as en-GB is in release 48.
		"a region preferred on its own side": {match("es_*_ES", "es_*_$!americas", 3) + rules48, clusters48},
		"one side rated closer than the other": {
			strings.Replace(rules48, self("es_*_$!americas", 4), self("es_*_$!americas", 3), 1),
			clusters48,
		},
		"a rule across at the across distance": {
			strings.Replace(rules48, self("es_*_*", 5), match("es_*_$americas", "es_*_$!americas", 5)+self("es_*_*", 5), 1),
			clusters48,
		},
		"a wildcard repeated after its family": {
			strings.Replace(rules48, self("es_*_*", 5), self("es_*_*", 5)+self("es_*_*", 5), 1),
			clusters48,
		},
		"a rule to another language": {match("ar_*_$maghreb", "fr_*_FR", 4) + rules48, clusters48},
		// 035 holds only PH here, and the tier never grades a group against
		// a region it holds.
		"a group rated against the one region it holds": {match("en_*_035", "en_*_PH", 1) + rules48, clusters48},
		"a script qualifier dropped": {
			strings.ReplaceAll(rules48, "zh_Hant_", "zh_*_"),
			strings.Replace(clusters48, `script: "Hant", `, "", 1),
		},
		"a language joins": {
			strings.Replace(rules48, default48, family("fr_*_", "americas", 4, 5)+default48, 1),
			strings.Replace(clusters48, es, es+` {language: "fr", variable: "$americas"},`, 1),
		},
		"a set difference": {
			`<matchVariable id="$1" value="019-MX"/>` + frJoins,
			strings.Replace(clusters48, es, es+` {language: "fr", variable: "$1"},`, 1),
		},
		// The rule rates MX and AR inside $1, so the split holds; with MX
		// subtracted it rates a pair across, refused below.
		"a pair inside rated as inside": {
			`<matchVariable id="$1" value="019"/>` + match("fr_*_MX", "fr_*_AR", 4) + frJoins,
			strings.Replace(clusters48, es, es+` {language: "fr", variable: "$1"},`, 1),
		},
		"an explicit oneway false": {
			strings.Replace(rules48, match("en_*_$!enUS", "en_*_GB", 3),
				`<languageMatch desired="en_*_$!enUS" supported="en_*_GB" distance="3" oneway="false"/>`, 1),
			clusters48,
		},
	}
	for name, c := range accepted {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			table, err := generate(doc(worldGroups), info(variables48+c.rules), "fixture")
			if err != nil {
				t.Fatal(err)
			}
			_, block, _ := bytes.Cut(flat(table), []byte("var regionClusters"))
			if got, want := string(bytes.TrimSpace(block)), "= []regionCluster{ "+c.want+" }"; got != want {
				t.Errorf("regionClusters %s, want %s", got, want)
			}
		})
	}
	without := func(rule string) string { return strings.Replace(rules48, rule, "", 1) }
	refused := map[string]struct{ rules, wantErr string }{
		"distances swapped": {
			strings.Replace(rules48, es48, family("es_*_", "americas", 5, 4), 1),
			"do not split es at $americas: es-001 => es-EU, on one side, rates 5, and es-002 => es-005, across it, rates 4",
		},
		"distances equal": {
			strings.Replace(rules48, es48, family("es_*_", "americas", 4, 4), 1),
			"do not split es at $americas: es-001 => es-EU, on one side, rates 4, and es-002 => es-005, across it, rates 4",
		},
		"complement missing": {
			without(self("es_*_$!americas", 4)),
			"do not split es at $americas: es-001 => es-EU, on one side, rates 5, and es-002 => es-005, across it, rates 5",
		},
		"wildcard missing": {
			without(self("es_*_*", 5)),
			"do not split es at $americas: es-001 => es-EU, on one side, rates 4, and es-002 => es-005, across it, rates 4",
		},
		"wildcard first": {
			self("es_*_*", 5) + rules48,
			"do not split es at $americas: es-001 => es-EU, on one side, rates 5, and es-002 => es-005, across it, rates 5",
		},
		"no rule rates a pair": {
			strings.TrimSuffix(without(self("es_*_*", 5)), default48),
			"no languageMatch rule rates es-002 against es-005",
		},
		"another language's self match": {
			strings.Replace(rules48, default48, self("fr_*_$maghreb", 4)+default48, 1),
			"do not split fr at $maghreb",
		},
		"only a rule to a region": {
			strings.Replace(rules48, en48, match("en_*_$!enUS", "en_*_GB", 3), 1),
			"do not split en at $enUS",
		},
		"a pair across rated as inside": {
			match("es_*_$!americas", "es_*_US", 4) + rules48,
			"es-002 => es-US, across it, rates 4",
		},
		"a pair across rated closer than inside": {
			match("es_*_$americas", "es_*_ES", 3) + rules48,
			"es-005 => es-ES, across it, rates 3",
		},
		"a pair on one side rated past across": {
			match("es_*_$!americas", "es_*_ES", 6) + rules48,
			"es-002 => es-ES, on one side, rates 6",
		},
		"a broader self match first": {
			strings.Replace(rules48, zh48, self("zh_*_*", 5)+zh48, 1),
			"do not split zh-Hant at $cnsar",
		},
		"the generic rule first": {
			default48 + rules48,
			"do not split ar at $maghreb",
		},
		"a oneway rule across": {
			`<languageMatch desired="es_*_ES" supported="es_*_$americas" distance="4" oneway="true"/>` + rules48,
			"es-ES => es-005, across it, rates 4",
		},
		"a set difference moves a region across": {
			`<matchVariable id="$1" value="019-MX"/>` + match("fr_*_MX", "fr_*_AR", 4) + frJoins,
			"fr-AR => fr-MX, across it, rates 4",
		},
		"a script-qualified pair across": {
			match("es_Latn_ES", "es_Latn_MX", 1) + rules48,
			"do not split es-Latn at $americas",
		},
		"a script rule for every language": {
			match("*_Latn_US", "*_Latn_GB", 1) + rules48,
			"do not split en-Latn at $enUS",
		},
		"a cluster with no region inside": {
			`<matchVariable id="$nowhere" value="QQ"/>` + strings.Replace(rules48, default48, family("de_*_", "nowhere", 4, 5)+default48, 1),
			"$nowhere for de: no pair of regions on one side or none across it",
		},
		// What regionClusters cannot hold.
		"undeclared variable": {self("pt_*_$iberia", 4) + rules48, "no matchVariable declares $iberia"},
		"every language":      {self("*_*_$americas", 4) + rules48, "names $americas for every language"},
		"malformed script":    {self("pt_latn_$americas", 4) + rules48, `script "latn"`},
		"variable not region": {self("pt_$americas", 4) + rules48, "want a variable only as the region"},
		"malformed region":    {self("pt_*_br", 4) + rules48, `region "br"`},
		"two-field side":      {match("pt_*_BR", "pt_BR", 4) + rules48, "want both sides"},
		"malformed distance":  {strings.Replace(rules48, `distance="5"`, `distance="far"`, 1), `distance "far"`},
		"no distance":         {`<languageMatch desired="pt_*_BR" supported="pt_*_PT"/>` + rules48, `distance ""`},
		"negative distance":   {strings.Replace(rules48, `distance="5"`, `distance="-1"`, 1), `distance "-1"`},
		"malformed oneway":    {`<languageMatch desired="pt_*_BR" supported="pt_*_PT" distance="4" oneway="yes"/>` + rules48, `oneway "yes"`},
		"overlapping script":  {strings.Replace(rules48, default48, family("zh_*_", "americas", 4, 5)+default48, 1), "two clusters for one language and script"},
		"second cluster for one language": {
			`<matchVariable id="$iberia" value="ES+PT"/>` + family("es_*_", "iberia", 4, 5) + rules48,
			"two clusters for one language and script",
		},
		"a variable no rule names": {`<matchVariable id="$iberia" value="ES+PT"/>` + rules48, "matchVariable $iberia: no languageMatch rule names it"},
	}
	for name, c := range refused {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := generate(doc(worldGroups), info(variables48+c.rules), "fixture")
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("generate = %v, want an error containing %q", err, c.wantErr)
			}
		})
	}
}

// serve returns a readFunc answering the two files with supplemental and
// languageInfo.
func serve(supplemental, languageInfo []byte) readFunc {
	return func(_ context.Context, f cldr.File) ([]byte, error) {
		switch f {
		case cldr.SupplementalData:
			return supplemental, nil
		case cldr.LanguageInfo:
			return languageInfo, nil
		}
		return nil, fmt.Errorf("no fixture for %s", f.Name())
	}
}

// TestRunWritesTheTable checks run writes exactly generate's table for the
// pinned files, its header naming the pinned release and both SHA-256 sums.
func TestRunWritesTheTable(t *testing.T) {
	t.Parallel()
	dir, supp, li := t.TempDir(), doc(worldGroups), info(variables48+rules48)
	if err := run(t.Context(), dir, serve(supp, li)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, tableFile))
	if err != nil {
		t.Fatal(err)
	}
	want, err := generate(supp, li, cldr.Release)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("run wrote:\n%s\nwant:\n%s", got, want)
	}
	for _, line := range []string{
		"// CLDR tag: " + cldr.Release + "\n",
		"// supplementalData.xml SHA-256: " + cldr.SHA256(supp) + "\n",
		"// languageInfo.xml SHA-256: " + cldr.SHA256(li) + "\n",
		"/cldr/" + cldr.Release + "/common/supplemental/supplementalData.xml\n",
		"/cldr/" + cldr.Release + "/common/supplemental/languageInfo.xml\n",
	} {
		if !bytes.Contains(got, []byte(line)) {
			t.Errorf("the table header lacks %q", line)
		}
	}
}

// TestRunRefusalWritesNothing checks a read the pin refuses, and a release
// generate refuses, leave no table.
func TestRunRefusalWritesNothing(t *testing.T) {
	t.Parallel()
	errPin := errors.New("languageInfo.xml has SHA-256 0, want the pinned 1")
	for name, read := range map[string]readFunc{
		"refused read": func(_ context.Context, f cldr.File) ([]byte, error) {
			if f == cldr.LanguageInfo {
				return nil, errPin
			}
			return doc(worldGroups), nil
		},
		"refused release": serve(doc(worldGroups), info(variables48+default48+rules48)),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := run(t.Context(), dir, read); err == nil {
				t.Fatal("run succeeded, want an error")
			}
			if _, err := os.Stat(filepath.Join(dir, tableFile)); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("run left %s after refusing (stat: %v)", tableFile, err)
			}
		})
	}
}

func TestAncestorsIsTheTransitiveClosure(t *testing.T) {
	t.Parallel()
	var d supplementalData
	if err := xml.Unmarshal(doc(fixtureGroups), &d); err != nil {
		t.Fatal(err)
	}
	got, err := ancestors(d.Groups)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"009": {"001"},
		"005": {"001", "019", "419"},
		"013": {"001", "019", "419"},
		"019": {"001"},
		"021": {"001", "019"},
		"419": {"001", "019"},
		"AQ":  {"001", "009", "QO"},
		"MX":  {"001", "013", "019", "419"},
		"PA":  {"001", "013", "019", "419"},
		"PZ":  {"001", "013", "019", "419"},
		"QO":  {"001", "009"},
		"US":  {"001", "019", "021"},
	}
	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("ancestors = %v, want %v", got, want)
	}
}

// TestMatchVariablesHoldTheirRegions checks each generated row lists,
// sorted, the regions its value takes in over worldGroups: 019 and every
// group and region inside it, a listed region no group places, a group
// whose every region is listed ($enUS's 021 and 029 here), and for a set
// difference no group left holding a region outside.
func TestMatchVariablesHoldTheirRegions(t *testing.T) {
	t.Parallel()
	vars := variables48 + customVariables + `<matchVariable id="$xx" value="ES+QQ"/>`
	rules := strings.Replace(rules48, default48, family("ww_*_", "1", 4, 5)+family("zz_*_", "x2", 4, 5)+family("de_*_", "xx", 4, 5)+default48, 1)
	table, err := generate(doc(worldGroups), info(vars+rules), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	_, block, _ := bytes.Cut(flat(table), []byte("var matchVariables"))
	block, _, _ = bytes.Cut(block, []byte("var regionClusters"))
	for _, row := range []string{
		`"$1": {"005", "013", "029", "419", "AR", "BR", "MX", "PA", "PR"},`,
		`"$americas": {"005", "013", "019", "021", "029", "419", "AR", "BR", "CA", "MX", "PA", "PR", "US"},`,
		`"$cnsar": {"HK", "MO"},`,
		`"$enUS": {"021", "029", "035", "057", "AS", "CA", "GU", "MH", "MP", "PH", "PR", "UM", "US", "VI"},`,
		`"$maghreb": {"DZ", "EH", "LY", "MA", "MR", "TN"},`,
		`"$x2": {"005", "021", "029", "AR", "BR", "CA", "MX", "PR", "US"},`,
		`"$xx": {"ES", "QQ"},`,
	} {
		if !bytes.Contains(block, []byte(row)) {
			t.Errorf("matchVariables lacks the row %s:\n%s", row, block)
		}
	}
	if n := bytes.Count(block, []byte(": {")); n != 7 {
		t.Errorf("matchVariables holds %d rows, want 7", n)
	}
}

func TestGenerateRefusesMalformed(t *testing.T) {
	t.Parallel()
	for name, groups := range map[string]string{
		"no groups":     ``,
		"bad group":     `<group type="0019" contains="MX"/>`,
		"bad member":    `<group type="019" contains="mx"/>`,
		"empty":         `<group type="019" contains=" "/>`,
		"self":          `<group type="019" contains="019"/>`,
		"cycle":         `<group type="019" contains="419"/><group type="419" contains="019"/>`,
		"cycle beneath": `<group type="001" contains="019"/><group type="019" contains="419"/><group type="419" contains="019"/>`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := generate(doc(groups), info(variables48+rules48), "fixture"); err == nil {
				t.Errorf("generate over %s succeeded, want a refusal", groups)
			}
		})
	}
	for name, c := range map[string]struct {
		li      []byte
		wantErr string
	}{
		"no variables":   {info(``), "declares no matchVariable"},
		"bad id":         {info(`<matchVariable id="maghreb" value="MA"/>`), "malformed id"},
		"bare $ id":      {info(`<matchVariable id="$" value="MA"/>`), "malformed id"},
		"complement id":  {info(`<matchVariable id="$!a" value="MA"/>`), "malformed id"},
		"spliced id":     {info(`<matchVariable id="$a&quot;b" value="MA"/>`), "malformed id"},
		"repeated id":    {info(`<matchVariable id="$a" value="MA"/><matchVariable id="$a" value="DZ"/>`), "declared twice"},
		"empty value":    {info(`<matchVariable id="$a" value=""/>`), `term ""`},
		"bad region":     {info(`<matchVariable id="$a" value="MA+dz"/>`), `term "dz"`},
		"negated region": {info(`<matchVariable id="$a" value="MA+$!b"/>`), `term "$!b"`},
		"leading '-'":    {info(`<matchVariable id="$a" value="-MX"/>`), `term ""`},
		"trailing '-'":   {info(`<matchVariable id="$a" value="019-"/>`), `term ""`},
		"two operators":  {info(`<matchVariable id="$a" value="019+-MX"/>`), `term ""`},
		"other operator": {info(`<matchVariable id="$a" value="019*MX"/>`), `term "019*MX"`},
		"no block":       {[]byte(`<supplementalData><languageMatching/></supplementalData>`), "holds 0 languageMatches"},
		"two blocks": {
			[]byte(`<supplementalData><languageMatching>` +
				`<languageMatches type="written_new"><matchVariable id="$a" value="MA"/></languageMatches>` +
				`<languageMatches type="written_new"><matchVariable id="$a" value="DZ"/></languageMatches>` +
				`</languageMatching></supplementalData>`),
			"holds 2 languageMatches",
		},
		"not xml": {[]byte(`<supplementalData><languageMatching>`), "parse " + cldr.LanguageInfo.Base()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := generate(doc(worldGroups), c.li, "fixture")
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("generate over %s = %v, want an error containing %q", c.li, err, c.wantErr)
			}
		})
	}
}

// flat joins b's fields with single spaces, since gofmt aligns map values.
func flat(b []byte) []byte { return bytes.Join(bytes.Fields(b), []byte(" ")) }
