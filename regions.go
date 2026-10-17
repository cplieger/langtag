package langtag

import (
	"slices"
	"strings"
)

// regionCluster is one of CLDR's paradigm clusters: regions of one language
// that CLDR rates closer among themselves than to any region outside (at
// most 4 against 5 at release-48). regionClusters in regiontable.go lists them,
// generated where the languageMatch rules of languageInfo.xml rate them so.
type regionCluster struct {
	// language is the [Tag.Language] both tags share.
	language string
	// script, when set, limits the cluster to tags resolving to that script.
	script string
	// variable names the cluster's languageInfo.xml matchVariable, whose
	// regions matchVariables in regiontable.go holds.
	variable string
}

// containsRegion reports whether parent is child or holds it in CLDR's
// territoryContainment. x/text's Region.Contains is not used: it ships CLDR
// 32, which puts BV, GS and UM in QO (language.CLDRVersion).
func containsRegion(parent, child string) bool {
	return parent == child || slices.Contains(territoryAncestors[child], parent)
}

// contains reports whether r falls inside the cluster, whose regions
// matchVariables lists sorted.
func (c regionCluster) contains(r string) bool {
	_, found := slices.BinarySearch(matchVariables[c.variable], r)
	return found
}

// unplacedRegion reports whether r is an ISO 3166-1 user-assigned code (AA,
// QM-QZ, XA-XZ, ZZ) that territoryContainment places in no group: ZZ is
// CLDR's unknown region and the rest are private use, so none names a
// regional standard. QO, QU and XK are user-assigned codes CLDR places, so
// they stay regions (Parse rewrites QU, the European Union, to EU). QA, the
// one Q code outside QM-QZ that x/text parses, is assigned and placed.
func unplacedRegion(r string) bool {
	userAssigned := r == "AA" || r == "ZZ" || strings.HasPrefix(r, "Q") || strings.HasPrefix(r, "X")
	return userAssigned && territoryAncestors[r] == nil
}

// statesRegion reports whether r, a [Tag.Region], names a region. It is the
// one predicate behind both the tier and [Tag.CoversRegion], so the two agree
// that es-ZZ states none.
func statesRegion(r string) bool {
	return r != "" && !unplacedRegion(r)
}

// acrossCluster reports whether a and b, two tags of one language whose
// scripts read as one, are two regional standards: both state a region CLDR
// places, neither region contains the other, and exactly one falls inside the
// language's cluster.
func acrossCluster(a, b Tag) bool {
	ra, rb := a.Region(), b.Region()
	if !statesRegion(ra) || !statesRegion(rb) ||
		containsRegion(ra, rb) || containsRegion(rb, ra) {
		return false
	}
	for _, c := range regionClusters {
		if c.language != a.macroBase || (c.script != "" && (a.script != c.script || b.script != c.script)) {
			continue
		}
		return c.contains(ra) != c.contains(rb)
	}
	return false
}
