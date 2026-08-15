package langtag

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
// Want and Have are primary language subtags after macrolanguage folding, the
// value [Tag.Language] reports. One entry therefore covers every spelling of
// the languages it names: a Want of "no" answers for nor, nob, no and nb
// alike, and no second entry is needed for the umbrella tag.
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

// builtinFallbacks is the curated cross-language table.
//
// Every entry is a claim about people, not about code, and each one is
// arguable. Three rules govern what belongs here.
//
// It must not be derivable. Anything that follows from macrolanguage folding
// or from script and region structure is already tier 0 through 2 and is a
// fact, not a judgment: nb against no, cmn against zh, arb against ar, tl
// against fil, sr-Cyrl against sr-Latn, and every regional variant.
//
// Its Kind must match its direction. Interchangeability is symmetric, because
// that is what the claim means. Shared literacy is not: a minority-language
// population reads the majority language and the reverse does not hold. An
// entry claiming Intelligible in one direction only is making the weaker claim
// under the stronger name, which is the error this table's shape exists to
// prevent.
//
// Its provenance must be checkable. CLDR's languageInfo.xml is the reference
// for whether upstream carries a relation, in which direction, and at what
// distance. Note that golang.org/x/text embeds a CLDR snapshot old enough to
// disagree with upstream (it does not know cs/sk or ca/es, and still carries
// the retired mk/bg), so the file itself is the source, never the library's
// matcher.
//
// The failure mode of a missing entry is that a track is left alone, which is
// the behavior a caller had before adopting this package. The failure mode of
// a wrong entry is a library silently rewritten into a language nobody asked
// for. Grow this table on request, not on inference.
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
	out := make([]Fallback, len(builtinFallbacks))
	copy(out, builtinFallbacks)
	return out
}
