# How the tiers work

This page is for a developer choosing a floor. It explains the tier order, the direction of a match, the script pairs that count as one language and how floors behave. The README's [tier table](../README.md#tiers) lists each tier with examples.

## The order ranks claims

The tiers run from the narrowest claim to the widest, not from the easiest substitute to read to the hardest. Reading effort does not rise steadily along the order, and `TierOtherScript` is where that shows. [CLDR](https://cldr.unicode.org), the Unicode Consortium's shared locale data, scores a generic same-language script change at a distance of 50. Every pair on the two tiers above it scores 4 to 20. So accepting `TierOtherScript` is a bigger ask for Chinese content than accepting `TierIntelligible` is for Norwegian.

The first three tiers follow from published standards data and hold no opinion. The two above them are curated judgments, kept apart because they make different claims. `TierIntelligible` says two languages are close enough that readers move between them, which holds in both directions. `TierSharedLiteracy` says only that readers of one are, as a population, literate in the other. That is a one-way fact about people, and it can allow a substitute in an unrelated language. Accepting the first does not accept the second.

## A match is directed

A comparison is directed, because a relationship between two languages can run one way only. `Prefer` names the language a person chose, so `p.Compare(have)` reads as that choice judging the offer. No call site holds two tags whose order could be swapped by mistake. A Catalan viewer can be offered a Spanish track at `TierSharedLiteracy`, and a Spanish viewer is never offered a Catalan one.

## Close scripts

A script difference is normally farther than a region difference. A few languages are written in two scripts whose readers are taught both, and for them the difference is no barrier. Those pairs sit at `TierSameLanguage`.

The list is derived from CLDR rather than judged. CLDR names only a handful of same-language script pairs against its generic 50. All but one are one-way transliteration rows, such as `ja-Latn` onto `ja-Jpan` and `hi-Latn` onto `hi-Deva`. A romanization feeding its native script is not two audiences reading each other. For the same reason, a one-way relationship between languages is shared literacy. One symmetric pair remains:

| Language | Scripts | Why | Provenance |
| --- | --- | --- | --- |
| `sr` | `Latn` and `Cyrl` | Serbian is written in both and Serbian schooling teaches both | CLDR distance 5, symmetric, against a generic 50 |

`CloseScripts` returns the list. Unlike the cross-language table, it is not replaceable, because it records an explicit symmetric distance in CLDR rather than a judgment about people. To keep Serbian Latin and Cyrillic apart anyway, compare the two tags' `Script` values yourself.

Three pairs are absent on purpose. CLDR does not name Simplified against Traditional Chinese, so that pair sits at the generic 50. The CLDR snapshot inside `x/text` rates `uz-Latn` against `uz-Cyrl` and `az-Latn` against `az-Cyrl` as close, and current CLDR names neither.

## Floors

You choose a floor, the widest tier you accept. `Match` reports whether a tag is within it, and `Best` returns every candidate at the closest tier within it.

`TierNone` means no relationship, so as a floor it would accept every language for every other. `Match` and `Best` therefore accept nothing at a floor of `TierNone` or beyond. `ParseTier` accepts every spelling `Tier.String` produces except `none`, in any letter case, plus the snake-case form such as `same_language`. For `none` and for any value it does not recognise, it returns `TierNone` and `ok == false`, so a mistyped setting matches nothing rather than everything.

For audio, stop at `TierOtherScript` or closer. Every cross-language entry is a claim about reading, and langtag cannot tell which kind of track you are matching.
