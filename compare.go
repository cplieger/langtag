package langtag

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
// An entry whose Kind is [SharedLiteracy] is applied in the stated direction
// only, whatever Both says. A population that reads a majority language does
// not put the majority population under the same obligation, so a reciprocal
// shared-literacy claim is always an authoring mistake rather than a policy.
func WithFallbacks(f []Fallback) *Comparer {
	c := &Comparer{fallbacks: make(map[string]map[string]fallbackHit, len(f))}
	add := func(want, have, reason string, tier Tier) {
		if want == "" || have == "" || want == have {
			// A self-directed or incomplete entry cannot change any answer:
			// identical languages are already TierIdentical or
			// TierSameLanguage. Dropping it keeps the table's meaning honest.
			return
		}
		if c.fallbacks[want] == nil {
			c.fallbacks[want] = make(map[string]fallbackHit, 2)
		}
		c.fallbacks[want][have] = fallbackHit{reason: reason, tier: tier}
	}
	for _, e := range f {
		tier := e.Kind.Tier()
		add(e.Want, e.Have, e.Reason, tier)
		if e.Both && e.Kind != SharedLiteracy {
			add(e.Have, e.Want, e.Reason, tier)
		}
	}
	return c
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
		// which is real work.
		if want.script == have.script {
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
func Match(want, have Tag, floor Tier) bool {
	return defaultComparer.Compare(want, have) <= floor
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
// floor is capped at [TierSharedLiteracy]: a floor of [TierNone] would
// otherwise accept every language as a substitute for every other, which is
// never what a caller means.
func BestWith[T any](c *Comparer, want Tag, candidates []T, tagOf func(T) Tag, floor Tier) (out []T, tier Tier, ok bool) {
	if floor >= TierNone {
		floor = TierSharedLiteracy
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
