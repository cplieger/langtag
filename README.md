# langtag

[![Go Reference](https://pkg.go.dev/badge/github.com/cplieger/langtag/v3.svg)](https://pkg.go.dev/github.com/cplieger/langtag/v3) [![Go version](https://img.shields.io/github/go-mod/go-version/cplieger/langtag)](https://github.com/cplieger/langtag/blob/main/go.mod) [![Mutation](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/cplieger/langtag/badges/mutation.json)](https://github.com/cplieger/langtag/issues?q=label%3Agremlins-tracker)

langtag matches a viewer's language choice to subtitle and audio tracks tagged in ISO 639 or BCP 47, and grades how close each substitute is.

A viewer picks a subtitle tagged `nob`, and the next episode carries only `nor`. String equality finds no match, though both name Norwegian. langtag replaces that check and the code tables you would keep by hand. It depends only on [`golang.org/x/text`](https://pkg.go.dev/golang.org/x/text), needs Go 1.27 or later and is licensed under Apache-2.0.

## Why use it

langtag is built for media software that must decide whether a differently tagged track can stand in for a stored language choice.

- `Parse` reads ISO 639-1, 639-2, 639-3 and BCP 47 tags, gives `ger` and `deu` one tag and reports that `nob` and `nor` share a language.
- `Compare` places a substitute on one of seven tiers, and you set the widest tier you accept.
- Matches between languages come from an eight-entry table, and `Reason` returns each entry's reason.
- At a floor of `TierIntelligible` or narrower, it offers only the same language or a close one. Tests check 32 unrelated pairs.
- Matches are directed, so a Catalan viewer can be offered Spanish at `TierSharedLiteracy`, and a Spanish viewer is never offered Catalan.
- A mistyped tier setting matches nothing, and `Compare` allocates nothing.

Consider the [`x/text/language` Matcher](https://pkg.go.dev/golang.org/x/text/language) if you need to pick which interface translation to serve from an `Accept-Language` header.

## Install

```sh
go get github.com/cplieger/langtag/v3@latest
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/cplieger/langtag/v3"
)

type subtitle struct {
	file string
	lang langtag.Tag
}

func main() {
	available := []subtitle{
		{"ep2.eng.srt", langtag.MustParse("eng")},
		{"ep2.nor.srt", langtag.MustParse("nor")},
		{"ep2.swe.srt", langtag.MustParse("swe")},
	}

	want, ok := langtag.Parse("nob") // what the viewer chose on episode 1
	if !ok {
		return
	}

	matches, tier, ok := langtag.Prefer(want).Best(available,
		func(s subtitle) langtag.Tag { return s.lang },
		langtag.TierSameLanguage)
	if !ok {
		fmt.Println("nothing close enough")
		return
	}
	fmt.Println(matches[0].file, "at tier", tier) // ep2.nor.srt at tier same-language
}
```

`Best` returns every candidate at the closest tier, in input order, so your own ranking, such as codec or a forced flag, picks among them. For a ranked list of choices, build one `Preference` per choice and call `Best` for each in turn.

The [package examples](https://pkg.go.dev/github.com/cplieger/langtag/v3#pkg-examples) show `Compare`, `Match`, `Reason` and `WithFallbacks`, and `go test` keeps them true.

## API

- `Parse`, `MustParse` and `Valid` read a raw identifier and reject placeholders such as `und` and `mul`, as [Parsing language tags](docs/parsing.md) describes. A `Tag` reports `String`, `Language`, `Script`, `StatedScript`, `Region`, `MultiScript` and `IsZero`, and `CoversScript` and `CoversRegion` report whether another tag gives it the script and region it asks for.
- `Prefer` binds the wanted language to the built-in table, and `Comparer.Prefer` binds it to a custom one. A `Preference` has `Compare`, `Reason`, `Match`, `Best`, `Want` and `String`.
- Seven `Tier` constants, with `Tier.String` and `ParseTier` for the configuration spellings.
- `names.English` gives a tag's English name, such as "Portuguese (Brazil)" or "Chinese (Traditional, Hong Kong SAR China)", and `names.Languages` lists every language code it names with that code's own English name, such as "tl" with "Tagalog".
- `Fallbacks` and `CloseScripts` return copies of the shipped tables. `WithFallbacks` replaces the table between languages, `ValidateFallbacks` names the entries it drops, and `Default` returns the built-in `Comparer`.
- `Fallback`, `CloseScript` and `Kind` are the table types.

`Best` is a generic method, which is why langtag needs Go 1.27. A generic method cannot appear in an interface, so to abstract over selection, wrap it in a non-generic method. A `Comparer` is immutable and safe for concurrent use. The full reference is on [pkg.go.dev](https://pkg.go.dev/github.com/cplieger/langtag/v3).

## Tiers

`Prefer(want).Compare(have)` reports one of seven tiers. You choose a floor, the widest tier you accept. `Match` and `Best` reject any candidate beyond it.

| Tier | Meaning | Examples |
| --- | --- | --- |
| `TierIdentical` | The same canonical tag | `ger` and `deu`, `nor` and `no`, `iw` and `he` |
| `TierSameLanguage` | One language a reader takes in without effort | `nob` and `nor`, `cmn` and `zh`, `es-MX` and `es-419`, `fr-CA` and `fr-FR`, `sr-Latn` and `sr-Cyrl` |
| `TierOtherRegion` | One language in two regional standards CLDR rates apart | `es-419` and `es-ES`, `pt-BR` and `pt-PT`, `en-US` and `en-GB` |
| `TierOtherScript` | One language in a script its readers are not taught alongside | `zh-Hans` and `zh-Hant`, `uz-Latn` and `uz-Cyrl` |
| `TierIntelligible` | Two different languages close enough to read across | `nb` and `nn`, `no` and `da`, `sr` and `hr`, `cs` and `sk`, `ms` and `id` |
| `TierSharedLiteracy` | A different language readers use because they are broadly literate in it | `ca` to `es` |
| `TierNone` | No relationship | `no` and `sv`, `hi` and `ur`, `pl` and `cs`, `gl` and `es` |

The first four tiers follow from standards data. `TierIntelligible` and `TierSharedLiteracy` come from the curated table, and accepting the first does not accept the second. The tiers run from the narrowest claim to the widest, and reading effort does not follow that order. CLDR, the Unicode locale data, rates Traditional Chinese for a Simplified reader on `TierOtherScript` a bigger step than Danish for a Norwegian on `TierIntelligible`.

For audio, stop at `TierOtherScript` or closer, because the table tiers are claims about reading. Danish and Norwegian are close on the page and far apart aloud. `TierNone` is not a usable floor, so `ParseTier` rejects `none` and any value it does not recognise. [How the tiers work](docs/tiers.md) has the detail.

## Unsupported by design

- Readers of some languages are also literate in another, as Catalan readers are in Spanish. langtag offers such a substitute only at `TierSharedLiteracy` and ships Catalan to Spanish as its one entry. To let English stand in for Tamil, add that entry with `WithFallbacks`.
- It cannot tell an audio track from a subtitle track. You pick the floor for each.
- It has no collation, formatting, content negotiation, `Accept-Language` parsing or numeric distance between tiers.
- It ships English names only, in the `names` package. For names in other languages, import [`x/text/language/display`](https://pkg.go.dev/golang.org/x/text/language/display), which adds about 2.5 MB to a binary.

[Non-goals](docs/non-goals.md) gives the reasons.

## Documentation

- [Parsing language tags](docs/parsing.md) lists what `Parse` accepts, rejects and canonicalizes.
- [How the tiers work](docs/tiers.md) explains the tier order, the direction of a match, the regional standards, the script pairs, unknown scripts, the coverage predicates and floors.
- [The cross-language table](docs/fallback-table.md) lists the eight shipped entries, how to replace them and how to propose one.
- [Non-goals](docs/non-goals.md) explains what langtag does not do and why.
- [Data sources](docs/data-sources.md) names the pinned CLDR and ISO 639-2 files behind the generated tables and how they are regenerated.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Disclaimer

This project is built with care and follows security best practices, but it is intended for personal / self-hosted use. No guarantees of fitness for production environments. Use at your own risk.

This project was built with AI-assisted tooling using [Claude](https://claude.com), [GPT](https://openai.com), and [Kiro](https://kiro.dev). The human maintainer defines architecture, supervises implementation, and makes all final decisions.

## License

Apache-2.0. See [LICENSE](LICENSE).
