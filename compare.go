package langtag

import "fmt"

// Comparer answers tier questions against a specific fallback table. Use it
// when the built-in cross-language judgments do not suit the deployment; the
// package-level [Compare], [Match] and [Best] use the built-in table.
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

// Compare reports how far the available tag have sits from the wanted tag
// want, using the built-in table.
//
// The argument order is load-bearing. Cross-language entries are directed, so
// Compare(want, have) and Compare(have, want) can differ: a Catalan viewer
// accepts a Spanish track, and a Spanish viewer does not accept a Catalan one.
// want is always the language a person chose; have is what the content offers.
func Compare(want, have Tag) Tier { return defaultComparer.Compare(want, have) }

// Compare reports how far the available tag have sits from the wanted tag
// want. See the package-level [Compare] for the argument-order contract.
func (c *Comparer) Compare(want, have Tag) Tier {
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

// Reason returns the recorded justification for a cross-language substitution,
// and ok=false when the pair is not one. Intended for explaining
// a surprising choice in a log line or a user-facing message.
func (c *Comparer) Reason(want, have Tag) (string, bool) {
	if want.IsZero() || have.IsZero() {
		return "", false
	}
	hit, ok := c.fallbacks[want.macroBase][have.macroBase]
	return hit.reason, ok
}

// Reason returns the recorded justification for a cross-language substitution
// under the built-in table. See [Comparer.Reason].
func Reason(want, have Tag) (string, bool) { return defaultComparer.Reason(want, have) }

// Match reports whether have is an acceptable stand-in for want at or within
// floor, using the built-in table.
//
// [TierNone] is not a usable floor and always reports false, in either
// position. It means "no relationship", so accepting it as a floor would accept
// every language as a substitute for every other. That matters because it is
// also what [ParseTier] returns for input it does not recognise: a mistyped
// configuration value therefore matches nothing rather than everything.
func Match(want, have Tag, floor Tier) bool {
	if floor >= TierNone {
		return false
	}
	return defaultComparer.Compare(want, have) <= floor
}

// Match reports whether have is an acceptable stand-in for want at or within
// floor, under this Comparer's table. See the package-level [Match] for the
// floor contract, which is identical: a floor of [TierNone] or beyond always
// reports false.
func (c *Comparer) Match(want, have Tag, floor Tier) bool {
	if floor >= TierNone {
		return false
	}
	return c.Compare(want, have) <= floor
}

// Best returns the candidates closest to want and the tier they matched at,
// using the built-in table. See [BestWith].
func Best[T any](want Tag, candidates []T, tagOf func(T) Tag, floor Tier) (out []T, tier Tier, ok bool) {
	return BestWith(defaultComparer, want, candidates, tagOf, floor)
}

// BestWith returns every candidate at the closest tier reached, the tier
// itself, and whether anything was within floor.
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
func BestWith[T any](c *Comparer, want Tag, candidates []T, tagOf func(T) Tag, floor Tier) (out []T, tier Tier, ok bool) {
	if floor >= TierNone {
		return nil, TierNone, false
	}
	best := TierNone
	for _, cand := range candidates {
		t := c.Compare(want, tagOf(cand))
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
