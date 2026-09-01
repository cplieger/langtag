package langtag

import "slices"

// Kind separates the two different claims a cross-language fallback can make.
// It decides which tier an entry lands on, so that a caller can accept one kind
// of substitution without accepting the other.
type Kind uint8

const (
	// KindUnset is the zero value, and it is not a usable Kind. An entry must
	// name its kind, because the two kinds land on different tiers and the
	// weaker claim must never inherit the stronger tier by default. Entries
	// carrying KindUnset are dropped by [WithFallbacks].
	KindUnset Kind = iota

	// Intelligible means the two languages are close enough that readers move
	// between them. Such a relationship runs both ways.
	Intelligible

	// SharedLiteracy means readers of the wanted language are, as a population,
	// literate in the other one, which may be entirely unrelated. Such a
	// relationship runs one way: the majority-language population does not
	// reciprocate.
	SharedLiteracy
)

// Tier returns the tier an entry of this Kind produces. An unset or unknown
// Kind yields [TierNone], so a malformed entry can never license a
// substitution.
func (k Kind) Tier() Tier {
	switch k {
	case Intelligible:
		return TierIntelligible
	case SharedLiteracy:
		return TierSharedLiteracy
	default:
		return TierNone
	}
}

// String returns the name of the kind.
func (k Kind) String() string {
	switch k {
	case Intelligible:
		return nameIntelligible
	case SharedLiteracy:
		return nameSharedLiteracy
	default:
		return "unset"
	}
}

// Fallback is one directed cross-language relationship: a reader who wants the
// language Want can use a track in the language Have.
//
// Want and Have are primary language subtags after macrolanguage folding —
// the value [Tag.Language] reports — so one entry covers every spelling of
// the languages it names.
//
// Both are bare strings, so a table author can write them in the wrong
// order and no type objects; ValidateFallbacks catches the mechanical
// mistakes (empty side, same-language pair, Kind/Both mismatch), and Reason
// forces every entry to state its argument in the claimed direction. A
// swapped pair whose Reason reads backwards is caught by review, not by a
// type: a table literal with named fields already shows both role names at
// every site.
type Fallback struct {
	// Want is the language subtag a reader asked for.
	Want string
	// Have is the language subtag they can use instead.
	Have string
	// Reason states why a reader of Want can use Have. It exists so that an
	// entry cannot be added without an argument, and so that a surprised user
	// can be shown one.
	Reason string
	// Provenance records where the claim comes from, so a later maintainer can
	// re-check it rather than inheriting it on trust.
	Provenance string
	// Kind decides the tier, and must agree with Both: an interchangeability
	// claim runs both ways, a shared-literacy claim runs one way.
	Kind Kind
	// Both records that the relationship holds in the reverse direction too.
	Both bool
}

// builtinFallbacks is the curated cross-language table. Every entry is a
// claim about people, not about code, and each is arguable. Three rules
// govern what belongs here: it must not be derivable (anything following
// from macrolanguage folding, script or region structure is already tiers
// 0-2 and is a fact, not a judgment); its Kind must match its direction
// (Intelligible is symmetric, SharedLiteracy is one-way — an entry claiming
// the former in one direction only understates itself); and its provenance
// must be checkable against CLDR's languageInfo.xml, never against
// golang.org/x/text's matcher, which embeds a CLDR snapshot old enough to
// disagree with upstream (it does not know cs/sk or ca/es, and still
// carries the retired mk/bg). A missing entry leaves a track alone; a wrong
// entry silently rewrites a language nobody asked for — grow this table on
// request, not on inference.
var builtinFallbacks = []Fallback{
	{
		Want: "no", Have: "nn", Kind: Intelligible, Both: true,
		Reason: "Bokmål and Nynorsk are the two written standards of Norwegian; " +
			"Norwegian schooling teaches both.",
		Provenance: "CLDR languageInfo.xml, nn<->nb and nn<->no, distance 20, symmetric.",
	},
	{
		Want: "no", Have: "da", Kind: Intelligible, Both: true,
		Reason: "Written Bokmål derives from Danish and the two remain close on " +
			"the page. This is a written-language claim only; the spoken " +
			"languages are much further apart.",
		Provenance: "CLDR languageInfo.xml, da<->no and da<->nb, distance 8, symmetric.",
	},
	{
		Want: "hr", Have: "bs", Kind: Intelligible, Both: true,
		Reason: "Croatian and Bosnian are mutually intelligible and share the " +
			"Latin script.",
		Provenance: "CLDR languageInfo.xml, hr<->bs, distance 4, symmetric.",
	},
	{
		Want: "cs", Have: "sk", Kind: Intelligible, Both: true,
		Reason: "Czech and Slovak are mutually intelligible in writing, and were " +
			"taught alongside each other for most of the twentieth century.",
		Provenance: "CLDR languageInfo.xml, cs<->sk, distance 20, symmetric.",
	},
	{
		Want: "ca", Have: "es", Kind: SharedLiteracy, Both: false,
		Reason: "Catalonia is officially bilingual and Spanish is compulsory in " +
			"schooling, so Catalan readers read Spanish. The reverse does not " +
			"hold, and the two are separate languages rather than variants.",
		Provenance: "CLDR languageInfo.xml, ca->es, distance 20, one-way. The same " +
			"distance and direction as gl->es, eu->es, cy->en and ga->en, which " +
			"are the rest of this family and are deliberately not shipped.",
	},
}

// Fallbacks returns a copy of the built-in table, so that a caller can read the
// shipped judgments, and amend a copy, without being able to mutate the
// package's own.
func Fallbacks() []Fallback {
	return slices.Clone(builtinFallbacks)
}
