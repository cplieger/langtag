package langtag

import "fmt"

// Comparer holds the fallback table a [Preference] is judged against. Use one
// when the built-in cross-language judgments do not suit the deployment: build
// it with [WithFallbacks], then bind a wanted language to it with
// [Comparer.Prefer]. [Prefer] binds against the built-in table.
//
// A Comparer is immutable after construction and safe for concurrent use.
type Comparer struct {
	// fallbacks maps a wanted language subtag to every language that may stand
	// in for it, flattened from the directed table at construction so a lookup
	// is one map hit rather than a scan. The value carries the tier the entry
	// produces alongside its reason, because the two kinds of entry land on
	// different tiers.
	fallbacks map[string]map[string]fallbackHit
}

// fallbackHit is what a table lookup yields: which tier the substitution sits
// at, and the argument for it.
type fallbackHit struct {
	reason string
	tier   Tier
}

// defaultComparer carries the built-in table. Built once at init because the
// flattening is pure and the result is read-only.
var defaultComparer = WithFallbacks(builtinFallbacks)

// Default returns the Comparer backed by the built-in table.
func Default() *Comparer { return defaultComparer }

// WithFallbacks returns a Comparer using the supplied cross-language table
// instead of the built-in one. Passing nil or an empty slice disables both
// cross-language tiers, so no substitution beyond [TierOtherScript] can occur
// at any floor.
//
// Entries name languages as [Tag.Language] reports them, so "no" rather than
// "nb" or "nor". An entry naming anything else simply never matches.
//
// Malformed entries are DROPPED rather than accommodated, so a table authoring
// mistake removes a substitution instead of licensing an unintended one. See
// [ValidateFallbacks] to find out which entries were rejected and why. An entry
// is malformed when it names no language on either side, names one language on
// both, carries no Kind, or claims a Kind that disagrees with its direction.
func WithFallbacks(f []Fallback) *Comparer {
	c := &Comparer{fallbacks: make(map[string]map[string]fallbackHit, len(f))}
	add := func(want, have, reason string, tier Tier) {
		if c.fallbacks[want] == nil {
			c.fallbacks[want] = make(map[string]fallbackHit, 2)
		}
		// Two entries can name one ordered pair, most easily when a symmetric
		// entry's reciprocal direction collides with an explicit one-way entry
		// for the same pair. Keep the FARTHER tier, so a later entry can only
		// ever narrow what a floor accepts. Overwriting would let table order
		// decide how close a pair is, and would silently promote a declared
		// shared-literacy claim onto the intelligible tier.
		if prev, exists := c.fallbacks[want][have]; exists && prev.tier >= tier {
			return
		}
		c.fallbacks[want][have] = fallbackHit{reason: reason, tier: tier}
	}
	for _, e := range f {
		if fallbackError(e) != "" {
			continue
		}
		tier := e.Kind.Tier()
		add(e.Want, e.Have, e.Reason, tier)
		// Only an interchangeability claim is reciprocated. A shared-literacy
		// claim runs one way even if an author set Both, because a majority
		// population does not read the minority language back.
		if e.Both && e.Kind == Intelligible {
			add(e.Have, e.Want, e.Reason, tier)
		}
	}
	return c
}

// ValidateFallbacks reports why each rejected entry in f was rejected, in
// order, and returns nil when every entry is usable. [WithFallbacks] drops the
// same entries silently, because a table is configuration rather than a
// runtime input; this is the way to find out what it dropped.
func ValidateFallbacks(f []Fallback) []error {
	var errs []error
	for i, e := range f {
		if msg := fallbackError(e); msg != "" {
			errs = append(errs, fmt.Errorf("fallback %d (%q -> %q): %s", i, e.Want, e.Have, msg))
		}
	}
	return append(errs, conflictingPairs(f)...)
}

// conflictingPairs reports ordered pairs that more than one entry in f claims at
// different tiers. Such a table is order-dependent, so the conflict is surfaced
// rather than resolved silently, even though WithFallbacks resolves it
// conservatively.
func conflictingPairs(f []Fallback) []error {
	type claim struct {
		tier  Tier
		index int
	}
	seen := make(map[[2]string]claim, len(f)*2)
	var errs []error
	record := func(want, have string, tier Tier, index int) {
		key := [2]string{want, have}
		if prev, exists := seen[key]; exists && prev.tier != tier {
			errs = append(errs, fmt.Errorf(
				"fallbacks %d and %d both claim %q -> %q, at %v and %v; the farther tier wins",
				prev.index, index, want, have, prev.tier, tier))
			return
		}
		seen[key] = claim{tier: tier, index: index}
	}
	for i, e := range f {
		if fallbackError(e) != "" {
			continue
		}
		record(e.Want, e.Have, e.Kind.Tier(), i)
		if e.Both && e.Kind == Intelligible {
			record(e.Have, e.Want, e.Kind.Tier(), i)
		}
	}
	return errs
}

// fallbackError returns the reason an entry is unusable, or the empty string
// when it is fine. The Kind-versus-direction rule is enforced here rather than
// only asserted in a test, so a caller-supplied table is held to the same
// contract as the built-in one.
func fallbackError(e Fallback) string {
	switch {
	case e.Want == "" || e.Have == "":
		return "both sides must name a language"
	case e.Want == e.Have:
		// Identical languages already match at TierIdentical or
		// TierSameLanguage, so such an entry could only ever be noise.
		return "both sides name the same language"
	case e.Kind == KindUnset:
		return "Kind must be set; an unset Kind would inherit the stronger tier"
	case e.Kind != Intelligible && e.Kind != SharedLiteracy:
		return "unknown Kind"
	case e.Kind == Intelligible && !e.Both:
		return "an Intelligible entry must be symmetric; a one-way claim is shared literacy"
	case e.Kind == SharedLiteracy && e.Both:
		return "a SharedLiteracy entry must not be symmetric; a majority population does not reciprocate"
	default:
		return ""
	}
}

// Preference is the language a person chose, bound to the fallback table that
// will judge substitutes for it. Construct one with [Prefer] for the built-in
// table or [Comparer.Prefer] for a custom one, then ask it about whatever the
// content offers.
//
// The construction carries the direction. Cross-language entries are directed
// — a Catalan viewer accepts a Spanish track, and a Spanish viewer does not
// accept a Catalan one — and two Tags side by side in an argument list can be
// transposed without anything noticing. A Preference names the wanted side
// once, at construction, so a comparison site has only one Tag to pass:
// p.Compare(have) reads as the person's choice judging the offer. Two
// preferences compared on adjacent lines can still be written in either
// order — what the role buys is that a transposition is named and visible at
// the site, not that it is impossible.
//
// The zero Preference is usable and fails closed. It prefers the zero Tag,
// which names no language, so Compare answers [TierNone] for every input,
// Reason answers ok=false, and Match and Best select nothing. The same
// direction holds for a Preference built from a nil *Comparer: it judges
// NOTHING acceptable, rather than silently inheriting the built-in table —
// a nil table is a caller bug, and the widest table this package ships is
// the one answer a bug must not receive.
type Preference struct {
	c    *Comparer
	want Tag
}

// Prefer binds want, the language a person chose, to the built-in
// cross-language table. Use [Comparer.Prefer] to judge against a custom table
// instead.
func Prefer(want Tag) Preference { return Preference{want: want, c: defaultComparer} }

// Prefer binds want, the language a person chose, to this Comparer's table.
func (c *Comparer) Prefer(want Tag) Preference { return Preference{want: want, c: c} }

// Compare reports how far the available tag have sits from the preferred
// language: the person's choice judging the offer. A Preference holding a nil
// table (possible only through a nil *Comparer, since [Prefer] binds the
// built-in one) answers [TierNone] for every input: fail closed, never widen.
func (p Preference) Compare(have Tag) Tier {
	if p.c == nil {
		return TierNone
	}
	return p.c.compare(p.want, have)
}

// Reason returns the recorded justification for the cross-language
// substitution that accepting have would be, and ok=false when the pair is not
// one. Intended for explaining a surprising choice in a log line or a
// user-facing message. A nil-table Preference answers ok=false for every
// input, matching Compare's fail-closed direction.
func (p Preference) Reason(have Tag) (string, bool) {
	if p.c == nil {
		return "", false
	}
	return p.c.reason(p.want, have)
}

// Match reports whether have is an acceptable stand-in for the preferred
// language at or within floor.
//
// [TierNone] is not a usable floor and always reports false. It means "no
// relationship", so accepting it as a floor would accept every language as a
// substitute for every other. That matters because it is also what [ParseTier]
// returns for input it does not recognise: a mistyped configuration value
// therefore matches nothing rather than everything.
func (p Preference) Match(have Tag, floor Tier) bool {
	if floor >= TierNone {
		return false
	}
	return p.Compare(have) <= floor
}

// Want returns the language this Preference was constructed with, so a caller
// holding only the Preference (the build-once idiom) can still log or display
// what was asked for.
func (p Preference) Want() Tag { return p.want }

// String renders the preference for a log line: the wanted tag's canonical
// form, or "<none>" for the zero Preference. The bound table is deliberately
// not rendered — two tables have no short faithful spelling.
func (p Preference) String() string {
	if p.want.IsZero() {
		return "<none>"
	}
	return p.want.String()
}

// Best returns every candidate at the closest tier reached from the preferred
// language, the tier itself, and whether anything was within floor.
//
// All returned candidates share one tier, because the caller is expected to
// rank within a language by criteria this package knows nothing about: codec,
// forced and hearing-impaired flags, track title, provider score. Input order
// is preserved so an existing tie-break stays deterministic.
//
// tagOf extracts the language from a candidate, which lets a caller pass its
// own type without building a parallel slice and mapping indices back. A
// candidate whose tag is the zero Tag never matches.
//
// A floor of [TierNone] or beyond selects nothing, and does NOT widen to the
// most permissive tier. An unrecognised configuration value parses to TierNone
// (see [ParseTier]), and the safe reading of an unusable floor is that no
// substitution was authorised, not that every substitution was.
//
// Best is a function rather than a method on [Preference] only because as of
// Go 1.26 a method cannot declare the type parameter tagOf needs; the
// preference sits where the receiver would. (Go 1.27's generic methods lift
// that restriction; revisit at the next major, not before — the free function
// stays correct either way.)
func Best[T any](p Preference, candidates []T, tagOf func(T) Tag, floor Tier) (out []T, tier Tier, ok bool) {
	if floor >= TierNone {
		return nil, TierNone, false
	}
	best := TierNone
	for _, cand := range candidates {
		t := p.Compare(tagOf(cand))
		if t > floor {
			continue
		}
		switch {
		case t < best:
			best = t
			out = append(out[:0], cand)
		case t == best:
			out = append(out, cand)
		}
	}
	if best > floor {
		return nil, TierNone, false
	}
	return out, best, true
}

// compare reports how far the available tag have sits from the wanted tag
// want. The directional hazard the public surface exists to prevent lives
// here, in one unexported place: every exported door routes through a
// [Preference], which named the wanted Tag when it was constructed.
func (c *Comparer) compare(want, have Tag) Tier {
	// A tag that names no language cannot justify a substitution, and that
	// includes two of them: "undetermined" equals "undetermined" as a string
	// while telling us nothing about what either track contains.
	if want.IsZero() || have.IsZero() {
		return TierNone
	}
	if want.canon == have.canon {
		return TierIdentical
	}
	if want.macroBase == have.macroBase {
		// One language. Either it is written the same way and only the region
		// differs, which changes nothing for a reader, or the script differs,
		// which is usually real work. The exception is a language written in two
		// scripts whose readers are taught both, which is no barrier at all.
		if want.script == have.script ||
			scriptsReadAsOne(want.macroBase, want.script, have.script) {
			return TierSameLanguage
		}
		return TierOtherScript
	}
	if hit, ok := c.fallbacks[want.macroBase][have.macroBase]; ok {
		return hit.tier
	}
	return TierNone
}

// reason returns the recorded justification for the want -> have substitution,
// and ok=false when the pair is not a cross-language entry.
func (c *Comparer) reason(want, have Tag) (string, bool) {
	if want.IsZero() || have.IsZero() {
		return "", false
	}
	hit, ok := c.fallbacks[want.macroBase][have.macroBase]
	return hit.reason, ok
}
