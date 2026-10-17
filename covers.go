package langtag

// CoversScript reports whether have's script satisfies t, the wanted tag.
// When t states a script, have must resolve to it; when t states none, have
// may resolve to t's resolved script or to its close pair for t's language
// ([CloseScripts]). An unknown script matches nothing: a have of unknown
// script covers no target of known script, and a target of unknown script is
// covered only by have == t. Apart from that identical-tag case it compares
// scripts only, never languages, and is false when either tag is zero.
func (t Tag) CoversScript(have Tag) bool {
	// A zero have needs no test of its own: its empty script equals no
	// resolved script and is in no close pair.
	if t.IsZero() {
		return false
	}
	// An unknown have needs no test either: past this guard t's script is
	// known, so it fails the equality, and closeScripts pairs only known
	// scripts (TestCloseScriptsCarryProvenance fails if a row adds Zzzz).
	if t.script == unknownScript {
		return t == have
	}
	if t.StatedScript() != "" {
		return have.script == t.script
	}
	return have.script == t.script || scriptsReadAsOne(t.macroBase, t.script, have.script)
}

// CoversRegion reports whether have's region satisfies t, the wanted tag: t
// states no region, or have states t's region or one that region contains in
// CLDR's territory containment, grouping regions included (es-UN covers
// es-MX). An unplaced region such as ZZ counts as none on either side, as it
// does for [TierOtherRegion]. It compares regions only, never languages, and
// is false when either tag is zero.
func (t Tag) CoversRegion(have Tag) bool {
	if t.IsZero() || have.IsZero() {
		return false
	}
	want := t.Region()
	if !statesRegion(want) {
		return true
	}
	// An unplaced have needs no test of its own: the table places it in no
	// group, so containsRegion holds for it only against itself, never want.
	return containsRegion(want, have.Region())
}
