# Non-goals

This page explains what langtag does not do and why, for a developer checking whether it fits a project.

## Unrelated languages stay out

langtag does not substitute an unrelated language at `TierIntelligible` or closer. [CLDR](https://cldr.unicode.org) is the Unicode Consortium's shared locale data. Its distance data rates Basque against Spanish, Welsh against English, Tamil against English and Belarusian against Russian as close, because those populations are broadly literate in the second language. That is accurate sociolinguistics and a poor default. A viewer who chose Tamil subtitles did not ask for English ones.

Claims of that shape live at `TierSharedLiteracy` alone, named for what they are and reached only by asking for that tier. The table ships one such entry, Catalan to Spanish. Extending it to the rest of that family is a decision for whoever runs the software, through `WithFallbacks`, rather than a default anybody inherits.

The first three tiers cannot express a relationship between two languages at all, and `TierIntelligible` is a hand-curated symmetric set. So no floor short of `TierSharedLiteracy` reaches an unrelated language. Tests list 33 such pairs and check that each stays at `TierNone` in both directions.

## Written, not spoken

Every cross-language entry is a claim about reading. Danish and Norwegian are close on the page and much further apart aloud, which is the clearest case. Software matching an audio track should stop at `TierOtherScript` or closer. The tiers above it are claims fit for subtitles, and langtag cannot tell which kind of track a caller is matching.

## Not an internationalization library

langtag has no collation, formatting, content negotiation or `Accept-Language` parsing. It gives no numeric distance between tiers, because a number invites arithmetic across steps that do not share a scale.

It ships no display names. The CLDR tables behind [`x/text/language/display`](https://pkg.go.dev/golang.org/x/text/language/display) add about 2.5 MB to a binary, so a caller that wants names imports that package directly.
