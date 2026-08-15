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
// Tiers 0 through 2 are derived from published standards data. The two above
// them are curated judgments, split because they make different claims:
// [TierIntelligible] says two languages are close enough to read across, and
// [TierSharedLiteracy] says only that readers of one are broadly literate in
// the other. A caller sets a floor and nothing beyond it is a match.
//
//	want, _ := langtag.Parse("nob")
//	have, _ := langtag.Parse("nor")
//	langtag.Compare(want, have) // TierSameLanguage
//
// # What this package deliberately will not do
//
// It will not substitute an unrelated language at the intelligible tier or
// below. CLDR's own distance data rates Basque against Spanish, Welsh against
// English and Tamil against English as close, because those populations are
// broadly literate in the second language. That is accurate sociolinguistics
// and a poor default: a viewer who chose Tamil subtitles did not ask for
// English ones. Claims of that shape live at [TierSharedLiteracy] alone, named
// for what they are, and the table ships one of them.
//
// Every cross-language entry is a claim about reading. Danish and Norwegian are
// close on the page and much further apart aloud. Software matching an audio
// track should stop at [TierOtherScript]; this package cannot tell which kind
// of track a caller is matching.
//
// It is not a general internationalization library. There is no collation, no
// formatting and no content negotiation. Display names live in the optional
// subpackage langtag/name, which is separate because its CLDR tables add
// roughly 2.5 MB to a binary.
package langtag
