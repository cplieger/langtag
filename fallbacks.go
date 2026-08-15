package langtag

// Fallback is one directed tier-3 relationship: a reader who wants the
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
	// Both records that the relationship holds in the reverse direction too.
	// It usually does not: a bilingual population reads the majority language,
	// while the majority population does not read theirs.
	Both bool
}

// builtinFallbacks is the tier-3 table.
//
// Every entry is a claim about people, not about code, and each one is
// arguable. Two rules govern what belongs here.
//
// It must not be derivable. Anything that follows from macrolanguage folding
// or from script and region structure is already tier 0 through 2 and is a
// fact, not a judgment: nb against no, cmn against zh, arb against ar, tl
// against fil, and every regional variant.
//
// It must be interchangeability, not shared literacy. CLDR's distance data
// rates a minority language as close to its surrounding majority language
// because the population reads both: Basque against Spanish, Welsh against
// English, Tamil against English, Belarusian against Russian. Those are
// one-way in CLDR's own data, which is the tell, and they are excluded. The
// one exception below was added by explicit decision and is marked as such.
//
// The failure mode of a missing entry is that a track is left alone, which is
// the behavior a caller had before adopting this package. The failure mode of
// a wrong entry is a library silently rewritten into a language nobody asked
// for. Grow this table on request, not on inference.
var builtinFallbacks = []Fallback{
	{
		Want: "no", Have: "nn", Both: true,
		Reason: "Bokmål and Nynorsk are the two written standards of Norwegian; " +
			"Norwegian schooling teaches both.",
	},
	{
		Want: "no", Have: "da", Both: true,
		Reason: "Written Bokmål derives from Danish and the two remain close in " +
			"writing, though not in speech.",
	},
	{
		Want: "hr", Have: "bs", Both: true,
		Reason: "Croatian and Bosnian are mutually intelligible and share the " +
			"Latin script.",
	},
	{
		Want: "ca", Have: "es", Both: false,
		Reason: "Catalonia is officially bilingual and Spanish is compulsory in " +
			"schooling, so Catalan readers read Spanish. The reverse does not " +
			"hold. Added by decision; CLDR carries no relation between the two.",
	},
}

// Fallbacks returns a copy of the built-in tier-3 table, so that a caller can
// read the shipped judgments, and amend a copy, without being able to mutate
// the package's own.
func Fallbacks() []Fallback {
	out := make([]Fallback, len(builtinFallbacks))
	copy(out, builtinFallbacks)
	return out
}
