// Package langtag answers whether one language track may stand in for the one
// that was asked for, and how far that substitution is.
//
// [Parse] canonicalizes ISO 639-1, 639-2 (both variants), 639-3 and BCP 47
// tags onto one BCP 47 form, so "nob" and "nor", or "ger" and "deu", compare.
// [Preference.Compare] grades a candidate as a [Tier]: tiers 0-2 come from
// standards data, [TierIntelligible] and [TierSharedLiteracy] are curated, and
// a caller's floor decides what matches. Relationships are directed; [Prefer]
// names the language the person chose.
//
// Shared literacy (Tamil readers reading English) lives at
// [TierSharedLiteracy] alone. Every cross-language entry is about reading, so
// audio matching should stop at [TierOtherScript]. There is no collation,
// formatting, content negotiation or display names (use
// golang.org/x/text/language/display).
package langtag
