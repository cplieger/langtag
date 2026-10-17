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
// language Want can use a track in the language Have. Want and Have are
// primary language subtags after macrolanguage folding (what [Tag.Language]
// reports), so one entry covers every spelling. Being bare strings, they can
// be swapped with no type objecting: [ValidateFallbacks] catches the
// mechanical mistakes (empty side, same-language pair, Kind/Both mismatch),
// and a Reason that reads backwards is left to review.
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

// builtinFallbacks is the curated cross-language table; each entry is an
// arguable claim about people. An entry must not be derivable (folding,
// script or region structure is already tiers 0-3), its Kind must match its
// direction, and its provenance must check against CLDR's languageInfo.xml,
// not x/text's matcher, whose older CLDR snapshot lacks cs/sk and ca/es and
// keeps the retired mk/bg. A wrong entry silently rewrites a language nobody
// asked for, so grow it on request, not on inference.
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
		Want: "sr", Have: "hr", Kind: Intelligible, Both: true,
		Reason: "Serbian and Croatian are mutually intelligible, and Serbian " +
			"schooling teaches the Latin script Croatian is written in.",
		Provenance: "CLDR languageInfo.xml release-48 lines 23 and 25, sh->hr and sh->sr, " +
			"distance 4. CLDR aliases sh to sr_Latn (supplementalMetadata.xml line 241) and " +
			"comments out its own sr rows (languageInfo.xml lines 22 and 24), so this " +
			"entry is langtag's reading of the sh rows as Serbian.",
	},
	{
		Want: "sr", Have: "bs", Kind: Intelligible, Both: true,
		Reason: "Serbian and Bosnian are mutually intelligible, and Serbian " +
			"schooling teaches the Latin script Bosnian is written in.",
		Provenance: "CLDR languageInfo.xml release-48 lines 21 and 25, sh->bs and sh->sr, " +
			"distance 4. CLDR aliases sh to sr_Latn (supplementalMetadata.xml line 241) and " +
			"comments out its own sr rows (languageInfo.xml lines 22 and 24), so this " +
			"entry is langtag's reading of the sh rows as Serbian.",
	},
	{
		Want: "cs", Have: "sk", Kind: Intelligible, Both: true,
		Reason: "Czech and Slovak are mutually intelligible in writing, and were " +
			"taught alongside each other for most of the twentieth century.",
		Provenance: "CLDR languageInfo.xml, cs<->sk, distance 20, symmetric.",
	},
	{
		Want: "ms", Have: "id", Kind: Intelligible, Both: true,
		Reason: "Malay and Indonesian are standards of one language, written in " +
			"the same Latin alphabet and spelling since 1972.",
		Provenance: "CLDR languageInfo.xml release-48 line 107, ms->id distance 30, and " +
			"line 261, id->ms distance 10: both directions. ISO 639-3 places ind " +
			"inside the msa macrolanguage.",
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
