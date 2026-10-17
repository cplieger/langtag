# How the tiers work

This page is for a developer choosing a floor. It explains the tier order, the direction of a match, the regional standards, the script pairs that count as one language, unknown scripts, the two coverage predicates and how floors behave. The README's [tier table](../README.md#tiers) lists each tier with examples.

## The order ranks claims

The tiers run from the narrowest claim to the widest, not from the easiest substitute to read to the hardest. Reading effort does not rise steadily along the order, and `TierOtherScript` is where that shows. [CLDR](https://cldr.unicode.org), the Unicode Consortium's shared locale data, scores a generic same-language script change at a distance of 50. Every pair on the two tiers above it scores 4 to 30. So accepting `TierOtherScript` is a bigger ask for Chinese content than accepting `TierIntelligible` is for Norwegian.

The first four tiers follow from published standards data and hold no opinion. The two above them are curated judgments, kept apart because they make different claims. `TierIntelligible` says two languages are close enough that readers move between them, which holds in both directions. `TierSharedLiteracy` says only that readers of one are, as a population, literate in the other. That is a one-way fact about people, and it can allow a substitute in an unrelated language. Accepting the first does not accept the second.

## A match is directed

A comparison is directed, because a relationship between two languages can run one way only. `Prefer` names the language a person chose, so `p.Compare(have)` reads as that choice judging the offer. No call site holds two tags whose order could be swapped by mistake. A Catalan viewer can be offered a Spanish track at `TierSharedLiteracy`, and a Spanish viewer is never offered a Catalan one.

## Regional standards

A region difference is normally no barrier, so `es-419` against `es-MX` and `fr-CA` against `fr-FR` sit at `TierSameLanguage`. CLDR singles out a few languages whose regions split into two written standards. It groups their regions into clusters and rates a pair 5 across a cluster's edge. Pairs on one side stay below that: the generic 4 that CLDR gives every other region difference, and 3 for British English offered to a reader of English outside the US cluster (`languageInfo.xml`, release 48, lines 13-16 and 440-456, the British preference at line 444). Those cross-cluster pairs sit at `TierOtherRegion`:

| Language | Cluster | Regions in it | Example across the edge |
| --- | --- | --- | --- |
| `ar` | Maghreb | `MA`, `DZ`, `TN`, `LY`, `MR`, `EH` | `ar-MA` and `ar-EG` |
| `en` | US English | `AS`, `CA`, `GU`, `MH`, `MP`, `PH`, `PR`, `UM`, `US`, `VI` | `en-US` and `en-GB` |
| `es` | The Americas | `019` | `es-419` and `es-ES` |
| `pt` | The Americas | `019` | `pt-BR` and `pt-PT` |
| `zh`, Traditional script only | Hong Kong and Macao | `HK`, `MO` | `zh-Hant-HK` and `zh-Hant-TW` |

A region is in a cluster when CLDR's list takes in every region it holds: a listed region takes in itself and every region it contains, so `019` takes in `419` and `MX`, and a region CLDR lists after a `-` is taken back out. Containment follows CLDR 48 (`supplementalData.xml` lines 1233-1280), grouping rows included, so `UN` contains `US`. It puts Bouvet Island (`BV`) and South Georgia (`GS`) in South America and the US Minor Outlying Islands (`UM`) in Micronesia (`057`), where the older CLDR copy inside `x/text` puts all three in Outlying Oceania (`QO`). A region straddling a cluster's edge, such as `029` for English or `015` for Arabic, counts as outside it, so the tier stays a subset of the pairs CLDR rates 5. The tier applies only when both tags state a region and neither region contains the other: `es-419` against `es-MX` stays `TierSameLanguage`, and so does a bare `pt` against `pt-BR`. A region CLDR places in no group counts as unstated: the unknown region `ZZ` and the private-use codes such as `AA` and `XA`, so `es-ZZ` against `es-MX` stays `TierSameLanguage`. `QO` and `XK` are private-use codes CLDR places, so they count, and `Parse` rewrites `QU`, the third, to the European Union's `EU`. It never hides a script difference, so `zh-Hant-HK` against `zh-Hans-TW` is `TierOtherScript`. The script that counts is the resolved one, so `zh-HK` against `zh-TW` is `TierOtherRegion`.

## Close scripts

A script difference is normally farther than a region difference. A few languages are written in two scripts whose readers are taught both, and for them the difference is no barrier. Those pairs sit at `TierSameLanguage`.

The list is derived from CLDR rather than judged. CLDR names only a handful of same-language script pairs against its generic 50. All but one are one-way transliteration rows, such as `ja-Latn` onto `ja-Jpan` and `hi-Latn` onto `hi-Deva`. A romanization feeding its native script is not two audiences reading each other. For the same reason, a one-way relationship between languages is shared literacy. One symmetric pair remains:

| Language | Scripts | Why | Provenance |
| --- | --- | --- | --- |
| `sr` | `Latn` and `Cyrl` | Serbian is written in both and Serbian schooling teaches both | CLDR distance 5, symmetric, against a generic 50 |

`CloseScripts` returns the list. Unlike the cross-language table, it is not replaceable, because it records an explicit symmetric distance in CLDR rather than a judgment about people. To keep Serbian Latin and Cyrillic apart anyway, compare the two tags' `Script` values yourself.

Three pairs are absent on purpose. CLDR does not name Simplified against Traditional Chinese, so that pair sits at the generic 50. The CLDR snapshot inside `x/text` rates `uz-Latn` against `uz-Cyrl` and `az-Latn` against `az-Cyrl` as close, and current CLDR names neither.

## Unknown scripts

A language CLDR gives no script, such as `cnr` or `ber`, reports `Zzzz`, and so does a tag stating `Zzzz`. An unknown script never counts as a script difference, so `cnr` against `cnr-Latn` and `ber` against `ber-Tfng` sit at `TierSameLanguage`. Two known scripts still differ: `cnr-Latn` against `cnr-Cyrl` is `TierOtherScript`. The region can still separate such a pair, so `en-Zzzz-US` against `en-GB` is `TierOtherRegion`. The Hong Kong and Macao cluster counts only Traditional tags, so `zh-Zzzz-TW` against `zh-Hant-HK` is `TierSameLanguage`.

## Coverage predicates

A tier grades how far apart two tags are. `Tag.CoversScript` and `Tag.CoversRegion` answer a narrower question: does the offered tag give the wanted one the script and the region it asked for? Both are directed, called on the wanted tag, and apart from the identical-tag rule below compare only script or only region, never the language. Both are false when either tag is the zero `Tag`.

`want.CoversScript(have)` holds when `want` states a script and `have` resolves to it, or when `want` states none and `have` resolves to `want`'s script or to its close pair. So a bare `sr` covers `sr-Cyrl` and `hr`, a bare `uz` does not cover `uz-Cyrl`, and `sr-Latn` does not cover `sr-Cyrl`. An unknown script matches nothing but the identical tag: a bare `cnr` covers itself but not `cnr-Latn` or `cnr-ME`, and `cnr-Cyrl` does not cover a bare `cnr`. That is stricter than the tiers, where an unknown script is no script difference.

`want.CoversRegion(have)` holds when `want` states no region, or when `have` states `want`'s region or one that region contains. Containment is the CLDR 48 table the regional standards use, grouping rows included. So `es-419` covers `es-MX`, `es-UN` covers `es-MX`, `es-MX` does not cover `es-419`, and `pt-BR` does not cover a bare `pt`. An unplaced region counts as none, as it does for the tier: an `es-ZZ` target covers `es-MX`, and an `es-MX` target does not cover `es-ZZ`.

## Floors

You choose a floor, the widest tier you accept. `Match` reports whether a tag is within it, and `Best` returns every candidate at the closest tier within it.

`TierNone` means no relationship, so as a floor it would accept every language for every other. `Match` and `Best` therefore accept nothing at a floor of `TierNone` or beyond. `ParseTier` accepts every spelling `Tier.String` produces except `none`, in any letter case, plus the snake-case form such as `same_language`. For `none` and for any value it does not recognise, it returns `TierNone` and `ok == false`, so a mistyped setting matches nothing rather than everything.

For audio, stop at `TierOtherScript` or closer. Every cross-language entry is a claim about reading, and langtag cannot tell which kind of track you are matching.
