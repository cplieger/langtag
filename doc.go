// Package langtag answers one question: may this language track stand in for
// the one that was asked for, and how far a substitution is that.
//
// The question comes up whenever software holds a language choice a person
// made and then meets content tagged differently. A media library is the
// motivating case: a viewer selects a subtitle tagged "nob" on one episode,
// the next episode carries only "nor", and string equality reports no match
// even though both name Norwegian Bokmål.
//
// # Tags are structured, not opaque
//
// Language identifiers arrive from files, transcoders and third-party APIs in
// every code system ever published: ISO 639-1 two-letter codes, both the
// bibliographic and terminological ISO 639-2 variants, ISO 639-3, and BCP 47
// tags with script and region subtags. [Parse] canonicalizes all of them onto
// one BCP 47 form, so the twenty ISO 639-2 duplicate pairs (ger/deu, fre/fra,
// chi/zho and the rest) and the deprecated codes (in, iw, ji) stop being
// distinct identities.
//
// # Distance is graded, not binary
//
// [Compare] reports a [Tier]: how far a candidate sits from what was wanted.
// Tiers 0 through 2 are derived from published standards data. Tier 3 is a
// curated table of judgments, and it is the only tier that encodes an opinion.
// A caller sets a floor and nothing beyond that floor is a match.
//
//	want, _ := langtag.Parse("nob")
//	have, _ := langtag.Parse("nor")
//	langtag.Compare(want, have) // TierSameLanguage
//
// # What this package deliberately will not do
//
// It will not substitute an unrelated language on the grounds that a
// population reads both. CLDR's own distance data rates Basque against
// Spanish, Welsh against English and Tamil against English as close, because
// those populations are broadly literate in the second language. That is
// accurate sociolinguistics and a poor substitution rule: a viewer who chose
// Tamil subtitles did not ask for English ones. Tiers 0 through 2 cannot
// express a cross-language substitution at all, so the whole family is
// excluded by construction rather than by a filter.
//
// It is not a general internationalization library. There is no collation, no
// formatting and no content negotiation. Display names live in the optional
// subpackage langtag/name, which is separate because its CLDR tables add
// roughly 2.5 MB to a binary.
package langtag
