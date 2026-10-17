package langtag

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/language"
)

// containmentCopy is supplementalData.xml's territoryContainment at CLDR
// release-48, lines 1234-1279, one row per group element in file order,
// grouping and deprecated rows included. It is the oracle the generated
// table and the region tier are checked against, never read from either.
// https://github.com/unicode-org/cldr/blob/release-48/common/supplemental/supplementalData.xml
var containmentCopy = []struct{ group, contains string }{
	{"001", "019 002 150 142 009"},
	{"001", "EU EZ UN"},
	{"001", "QU"},
	{"011", "BF BJ CI CV GH GM GN GW LR ML MR NE NG SH SL SN TG"},
	{"013", "BZ CR GT HN MX NI PA SV"},
	{"014", "BI DJ ER ET IO KE KM MG MU MW MZ RE RW SC SO SS TF TZ UG YT ZM ZW"},
	{"142", "145 143 030 034 035"},
	{"143", "TM TJ KG KZ UZ"},
	{"145", "AE AM AZ BH CY GE IL IQ JO KW LB OM PS QA SA SY TR YE"},
	{"145", "NT YD"},
	{"015", "DZ EG EH LY MA SD TN EA IC"},
	{"150", "154 155 151 039"},
	{"151", "BG BY CZ HU MD PL RO RU SK UA"},
	{"151", "SU"},
	{"154", "GG IM JE AX DK EE FI FO GB IE IS LT LV NO SE SJ CQ"},
	{"155", "AT BE CH DE FR LI LU MC NL"},
	{"155", "DD FX"},
	{"017", "AO CD CF CG CM GA GQ ST TD"},
	{"017", "ZR"},
	{"018", "BW LS NA SZ ZA"},
	{"019", "021 013 029 005"},
	{"019", "003 419"},
	{"002", "015 011 017 014 018"},
	{"002", "202"},
	{"202", "011 017 014 018"},
	{"021", "BM CA GL PM US"},
	{"029", "AG AI AW BB BL BQ BS CU CW DM DO GD GP HT JM KN KY LC MF MQ MS PR SX TC TT VC VG VI"},
	{"029", "AN"},
	{"003", "021 013 029"},
	{"030", "CN HK JP KP KR MN MO TW"},
	{"035", "BN ID KH LA MM MY PH SG TH TL VN"},
	{"035", "BU TP"},
	{"039", "AD AL BA ES GI GR HR IT ME MK MT RS PT SI SM VA XK"},
	{"039", "CS YU"},
	{"419", "013 029 005"},
	{"005", "AR BO BR BV CL CO EC FK GF GS GY PE PY SR UY VE"},
	{"053", "AU CC CX HM NF NZ"},
	{"054", "FJ NC PG SB VU"},
	{"057", "FM GU KI MH MP NR PW UM"},
	{"061", "AS CK NU PF PN TK TO TV WF WS"},
	{"034", "AF BD BT IN IR LK MV NP PK"},
	{"009", "053 054 057 061 QO"},
	{"QO", "AQ AC CP DG TA"},
	{"EU", "AT BE CY CZ DE DK EE ES FI FR GR HR HU IE IT LT LU LV MT NL PL PT SE SI SK BG RO"},
	{"EZ", "AT BE CY DE EE ES FI FR GR IE IT LT LU LV MT NL PT SI SK"},
	{"UN", "AD AE AF AG AL AM AO AR AT AU AZ BA BB BD BE BF BG BH BI BJ BN BO BR BS BT BW BY BZ CA CD CF CG CH CI CL CM CN CO CR CU CV CY CZ DE DJ DK DM DO DZ EC EE EG ER ES ET FI FJ FM FR GA GB GD GE GH GM GN GQ GR GT GW GY HN HR HT HU ID IE IL IN IQ IR IS IT JM JO JP KE KG KH KI KM KN KP KR KW KZ LA LB LC LI LK LR LS LT LU LV LY MA MC MD ME MG MH MK ML MM MN MR MT MU MV MX MW MY MZ NA NE NG NI NL NO NR NP NZ OM PA PE PG PH PK PL PT PW PY QA RO RS RU RW SA SB SC SD SE SG SI SK SL SM SN SO SR SS ST SV SY SZ TD TG TH TJ TL TM TN TO TR TT TV TZ UA UG US UY UZ VC VE VN VU WS YE ZA ZM ZW"},
}

// modelContains is the transitive closure of containmentCopy, computed to a
// fixpoint: modelContains[child][parent] holds when parent contains child,
// directly or through a group. modelHolds adds reflexivity.
var modelContains = func() map[string]map[string]bool {
	in := map[string]map[string]bool{}
	add := func(child, parent string) bool {
		if in[child] == nil {
			in[child] = map[string]bool{}
		}
		if in[child][parent] {
			return false
		}
		in[child][parent] = true
		return true
	}
	for _, row := range containmentCopy {
		for child := range strings.FieldsSeq(row.contains) {
			add(child, row.group)
		}
	}
	for changed := true; changed; {
		changed = false
		for child, parents := range in {
			for parent := range parents {
				for grand := range in[parent] {
					changed = add(child, grand) || changed
				}
			}
		}
	}
	return in
}()

func modelHolds(parent, child string) bool {
	return parent == child || modelContains[child][parent]
}

// TestRegionTableMatchesCopy compares the generated table, region by region,
// with the closure of the transcribed rows: a hand edit of regiontable.go, or
// a regeneration that moves a copied row, fails naming the region.
func TestRegionTableMatchesCopy(t *testing.T) {
	t.Parallel()
	for _, r := range slices.Sorted(maps.Keys(modelContains)) {
		want := slices.Sorted(maps.Keys(modelContains[r]))
		if got := territoryAncestors[r]; !slices.Equal(got, want) {
			t.Errorf("territoryAncestors[%q] = %v, want %v", r, got, want)
		}
	}
	for r := range territoryAncestors {
		if modelContains[r] == nil {
			t.Errorf("territoryAncestors holds %q, which the copy puts in no group", r)
		}
	}
}

// matchVariablesCopy is the regions languageInfo.xml's matchVariable
// entries at CLDR release-48, lines 13-16, hold, sorted: the oracle for the
// generated matchVariables. $americas, 019, is americasCopy bar PZ, which
// territoryContainment places in no group.
// https://github.com/unicode-org/cldr/blob/release-48/common/supplemental/languageInfo.xml
var matchVariablesCopy = map[string][]string{
	"$enUS":     {"AS", "CA", "GU", "MH", "MP", "PH", "PR", "UM", "US", "VI"},
	"$cnsar":    {"HK", "MO"},
	"$americas": slices.Sorted(slices.Values(slices.DeleteFunc(slices.Clone(americasCopy), func(r string) bool { return r == "PZ" }))),
	"$maghreb":  {"DZ", "EH", "LY", "MA", "MR", "TN"},
}

// TestMatchVariablesMatchCopy compares the generated cluster members with
// the transcription: a refresh that moves a member fails here until the
// copy above is updated by hand, which is the review.
func TestMatchVariablesMatchCopy(t *testing.T) {
	t.Parallel()
	if !maps.EqualFunc(matchVariables, matchVariablesCopy, slices.Equal) {
		t.Errorf("matchVariables = %v, want the copy's %v", matchVariables, matchVariablesCopy)
	}
}

// clustersCopy is the bindings languageInfo.xml's languageMatch rules make
// at CLDR release-48, lines 440-455: the oracle for the generated
// regionClusters.
// https://github.com/unicode-org/cldr/blob/release-48/common/supplemental/languageInfo.xml
var clustersCopy = []regionCluster{
	{language: "ar", variable: "$maghreb"},
	{language: "en", variable: "$enUS"},
	{language: "es", variable: "$americas"},
	{language: "pt", variable: "$americas"},
	{language: "zh", script: "Hant", variable: "$cnsar"},
}

// TestRegionClustersMatchCopy compares the generated bindings with the
// transcription: a refresh that moves a variable to another language or
// script, adds one or drops one fails here until the copy above is updated
// by hand, which is the review.
func TestRegionClustersMatchCopy(t *testing.T) {
	t.Parallel()
	if !slices.Equal(regionClusters, clustersCopy) {
		t.Errorf("regionClusters = %+v, want the copy's %+v", regionClusters, clustersCopy)
	}
}

// americasCopy is CLDR release-48's 019 and everything it contains,
// expanded by hand from supplementalData.xml lines 1238-1269 (AN from the
// deprecated row at 1261, PZ aliased to PA at supplementalMetadata.xml line
// 577).
var americasCopy = []string{
	"019", "003", "419", "021", "013", "029", "005",
	"BM", "CA", "GL", "PM", "US", // 021
	"BZ", "CR", "GT", "HN", "MX", "NI", "PA", "SV", "PZ", // 013
	"AG", "AI", "AW", "BB", "BL", "BQ", "BS", "CU", "CW", "DM", "DO", "GD", "GP", "HT",
	"JM", "KN", "KY", "LC", "MF", "MQ", "MS", "PR", "SX", "TC", "TT", "VC", "VG", "VI", "AN", // 029
	"AR", "BO", "BR", "BV", "CL", "CO", "EC", "FK", "GF", "GS", "GY", "PE", "PY", "SR", "UY", "VE", // 005
}

// TestAmericasIsTheModelClosure checks the two transcriptions against each
// other: the regions whose parsed tag the model puts in 019 are exactly
// americasCopy.
func TestAmericasIsTheModelClosure(t *testing.T) {
	t.Parallel()
	var got []string
	for _, r := range parsedRegions() {
		if modelHolds("019", MustParse("es-"+r).Region()) {
			got = append(got, r)
		}
	}
	slices.Sort(got)
	want := slices.Sorted(slices.Values(americasCopy))
	if !slices.Equal(got, want) {
		t.Errorf("regions the model puts in 019 = %v, want americasCopy %v", got, want)
	}
}

// TestOtherRegionEveryPair grades every pair of regions x/text parses, both
// ways, for each clustered language, against a model built from
// containmentCopy, americasCopy and the members below, never from
// regionClusters, matchVariables or territoryAncestors: other-region exactly
// when CLDR places both regions, neither holds the other and one side is in
// the cluster. The en members add the four codes supplementalMetadata.xml
// lines 570-584 alias to UM.
func TestOtherRegionEveryPair(t *testing.T) {
	t.Parallel()
	clusters := []struct {
		prefix string
		inside []string
	}{
		{"ar", []string{"MA", "DZ", "TN", "LY", "MR", "EH"}},
		{"en", []string{"AS", "CA", "GU", "MH", "MP", "PH", "PR", "UM", "US", "VI", "JT", "MI", "PU", "WK"}},
		{"es", americasCopy},
		{"pt", americasCopy},
		{"zh-Hant", []string{"HK", "MO"}},
	}
	regions := parsedRegions()
	for _, c := range clusters {
		tags := make([]Tag, len(regions))
		for i, r := range regions {
			tag, ok := Parse(c.prefix + "-" + r)
			if !ok {
				t.Fatalf("Parse(%q) failed for a region x/text accepts", c.prefix+"-"+r)
			}
			tags[i] = tag
		}
		for i, a := range tags {
			t.Run(c.prefix+"-"+regions[i], func(t *testing.T) {
				t.Parallel()
				aInside := slices.Contains(c.inside, regions[i])
				for j, b := range tags {
					want := TierSameLanguage
					switch {
					case a == b:
						want = TierIdentical
					case modelUnplaced(a.Region()) || modelUnplaced(b.Region()):
					case modelHolds(a.Region(), b.Region()) || modelHolds(b.Region(), a.Region()):
					case aInside != slices.Contains(c.inside, regions[j]):
						want = TierOtherRegion
					}
					if got := Prefer(a).Compare(b); got != want {
						t.Errorf("Prefer(%v).Compare(%v) = %v, want %v", a, b, got, want)
					}
				}
			})
		}
	}
}

// modelUnplaced reports a region the tier treats as unstated: a code x/text
// marks private use (ISO 3166-1's user-assigned AA, QM-QZ, XA-XZ and ZZ) that
// the transcribed containment places in no group. QO, QU and XK are
// user-assigned codes CLDR places, so they stay regions.
func modelUnplaced(r string) bool {
	pr, err := language.ParseRegion(r)
	return err == nil && pr.IsPrivateUse() && modelContains[r] == nil
}

// parsedRegions lists every two-letter and three-digit region code
// golang.org/x/text parses, deduplicated by its canonical spelling (840 is US).
func parsedRegions() []string {
	var out []string
	add := func(code string) {
		r, err := language.ParseRegion(code)
		if err == nil && !slices.Contains(out, r.String()) {
			out = append(out, r.String())
		}
	}
	for a := 'A'; a <= 'Z'; a++ {
		for b := 'A'; b <= 'Z'; b++ {
			add(string([]rune{a, b}))
		}
	}
	for n := range 1000 {
		add(fmt.Sprintf("%03d", n))
	}
	return out
}
