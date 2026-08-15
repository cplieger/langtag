# langtag

[![Go Reference](https://pkg.go.dev/badge/github.com/cplieger/langtag.svg)](https://pkg.go.dev/github.com/cplieger/langtag)
[![Go version](https://img.shields.io/github/go-mod/go-version/cplieger/langtag)](https://github.com/cplieger/langtag/blob/main/go.mod)
[![Test coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/cplieger/langtag/badges/coverage.json)](https://github.com/cplieger/langtag/actions/workflows/coverage.yml)
[![Mutation](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/cplieger/langtag/badges/mutation.json)](https://github.com/cplieger/langtag/issues?q=label%3Agremlins-tracker)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/14097/badge)](https://www.bestpractices.dev/projects/14097)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/cplieger/langtag/badge)](https://scorecard.dev/viewer/?uri=github.com/cplieger/langtag)

> May this language track stand in for the one that was asked for, and how far a substitution is that

Software that stores a language choice a person made will eventually meet content tagged
differently. A viewer picks a subtitle tagged `nob` on one episode, the next episode carries only
`nor`, and string equality reports no match even though both name Norwegian Bokmål.

`langtag` canonicalizes language identifiers from any published code system onto one BCP 47 form,
then grades the distance between what was wanted and what is available on a five-step scale. The
only runtime dependency is [`golang.org/x/text`](https://pkg.go.dev/golang.org/x/text).

## Install

`go get github.com/cplieger/langtag@latest`

## Usage

```go
package main

import (
	"fmt"

	"github.com/cplieger/langtag"
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

	matches, tier, ok := langtag.Best(want, available,
		func(s subtitle) langtag.Tag { return s.lang },
		langtag.TierSameLanguage)
	if !ok {
		fmt.Println("nothing close enough")
		return
	}
	fmt.Println(matches[0].file, "at tier", tier) // ep2.nor.srt at tier same-language
}
```

## Parsing

`Parse` accepts ISO 639-1, ISO 639-2 in either the bibliographic or terminological variant,
ISO 639-3, and BCP 47 tags, in any letter case, with surrounding whitespace. It returns
`ok == false` rather than an error, because ignoring the tag is the only available recovery.

Canonicalization collapses the code systems that spell one language several ways:

| Input | Canonical | Language |
| --- | --- | --- |
| `ger`, `deu`, `de` | `de` | `de` |
| `chi`, `zho`, `zh` | `zh` | `zh` |
| `iw` | `he` | `he` |
| `nor`, `no` | `no` | `no` |
| `nob`, `nb` | `nb` | `no` |
| `cmn` | `cmn` | `zh` |

`Tag.Language` reports the primary subtag after macrolanguage folding, so `nob` and `nor` share
the language `no` while keeping distinct tags. That value is the right key for a cache or a
learned-preference map, because it does not fragment across spellings.

These inputs are rejected: the empty string, the placeholder subtags that name no language
(`und`, `zxx`, `mul`, `mis`), private-use subtags (`qaa` through `qtz`), and anything the IANA
Language Subtag Registry does not know. Variant and extension subtags (`-u-`, `-t-`, `-x-`, and
forms like `ca-valencia`) are discarded, because none of them changes which language a track is
in.

The zero `Tag` matches nothing, including another zero `Tag`. Two tracks both labelled
"undetermined" are not known to share a language.

## Tiers

`Compare(want, have)` reports how far the available language sits from the wanted one. Lower is
closer. A caller passes the highest tier it will accept as a floor.

| Tier | Meaning | Examples |
| --- | --- | --- |
| `TierIdentical` | The same language, the same canonical tag | `ger` and `deu`, `nor` and `no`, `iw` and `he` |
| `TierSameLanguage` | One language, written the same way, differing at most in region | `nob` and `nor`, `cmn` and `zh`, `es-ES` and `es-419`, `en-GB` and `en-US` |
| `TierOtherScript` | One language in a different script | `zh-Hans` and `zh-Hant`, `sr-Cyrl` and `sr-Latn` |
| `TierIntelligible` | Two different languages that are close enough to read across | `nb` and `nn`, `no` and `da`, `hr` and `bs`, `cs` and `sk` |
| `TierSharedLiteracy` | A different language readers use because they are broadly literate in it | `ca` to `es` |
| `TierNone` | No relationship worth acting on | `no` and `sv`, `hi` and `ur`, `pl` and `cs`, `gl` and `es` |

The first three tiers follow from published standards data and hold no opinion. The two above
them are curated judgments, and they are separate tiers because they make different claims.
`TierIntelligible` says two languages are close enough that readers move between them, which is a
symmetric property of the languages. `TierSharedLiteracy` says only that readers of one are, as a
population, literate in the other, which is a one-way fact about people and licenses a
substitution to a possibly unrelated language. Accepting the first does not accept the second.

`Compare` is **not** commutative. Cross-language relationships are directed, so the argument order
matters: `want` is the language a person chose, `have` is what the content offers. A Catalan
viewer accepts a Spanish track; a Spanish viewer does not accept a Catalan one.

## The cross-language table

Five shipped relationships, each a claim about people rather than about code, each one arguable.
`Fallbacks` returns the table and `Reason` returns the justification for a given pair, so a
surprised user can be shown why a substitution happened.

| Wanted | Accepted | Kind | Both ways | Why |
| --- | --- | --- | --- | --- |
| `no` | `nn` | intelligible | yes | Bokmål and Nynorsk are the two written standards of Norwegian; Norwegian schooling teaches both |
| `no` | `da` | intelligible | yes | Written Bokmål derives from Danish and the two remain close on the page |
| `hr` | `bs` | intelligible | yes | Croatian and Bosnian are mutually intelligible and share the Latin script |
| `cs` | `sk` | intelligible | yes | Czech and Slovak are mutually intelligible in writing |
| `ca` | `es` | shared-literacy | no | Catalonia is officially bilingual and Spanish is compulsory in schooling |

Entries name languages the way `Tag.Language` reports them, so one entry covers every spelling:
a wanted language of `no` answers for `nor`, `nob`, `no` and `nb` alike.

The `Kind` decides the tier, and it has to agree with the direction. An interchangeability claim
runs both ways, because that is what the claim means. A shared-literacy claim runs one way, because
a majority-language population does not reciprocate. A test enforces both, so an entry cannot make
the weaker claim under the stronger name.

Every entry also carries a `Provenance` field naming where the claim can be re-checked, which for
all five is [CLDR's `languageInfo.xml`](https://github.com/unicode-org/cldr/blob/main/common/supplemental/languageInfo.xml).

`WithFallbacks` replaces the table for a deployment that disagrees, and `WithFallbacks(nil)`
disables both cross-language tiers.

### A note on verifying these claims

`golang.org/x/text` embeds a CLDR snapshot old enough to disagree with upstream: its matcher does
not know `cs`/`sk` or `ca`/`es`, and still carries a `mk`/`bg` relation that CLDR has since
retired. So the reference for whether upstream carries a relation, in which direction, and at what
distance, is `languageInfo.xml` itself, never the library's matcher. This package uses `x/text`
only for parsing and canonicalization, which come from the IANA Language Subtag Registry and are
not affected.

## What this package will not do

**It will not substitute an unrelated language at the intelligible tier or below.** CLDR's
distance data rates Basque against Spanish, Welsh against English, Tamil against English and
Belarusian against Russian as close, because those populations are broadly literate in the second
language. That is accurate sociolinguistics and a poor default: a viewer who chose Tamil subtitles
did not ask for English ones.

Claims of that shape live at `TierSharedLiteracy`, alone, named for what they are, and reachable
only by asking for that tier specifically. The table ships one such entry. Extending it to the
rest of the family is a decision for whoever runs the software, through `WithFallbacks`, rather
than a default anybody inherits.

Everything below that tier is structural. Tiers 0 through 2 cannot express a cross-language
relationship at all, and the intelligible tier is a hand-curated symmetric set, so no amount of
loosening a floor short of `shared-literacy` can reach an unrelated language. Tests enumerate 33
such pairs and assert every one stays at `TierNone` at every floor.

Also absent, deliberately: collation, formatting, content negotiation, `Accept-Language` parsing,
and any numeric distance between tiers (a number invites arithmetic across steps that are not
commensurable). Display names live in the optional `langtag/name` subpackage, kept separate
because its CLDR tables add roughly 2.5 MB to a binary.

### Written, not spoken

Every cross-language entry is a claim about reading. Danish and Norwegian are close on the page
and much further apart aloud, which is the clearest case. Software matching an **audio** track
should stop at `TierOtherScript` or below; the tiers above it are subtitle-grade claims and this
package cannot tell which kind of track a caller is matching.

## Adding to the table

A missing entry means a track is left alone, which is what a caller had before adopting this
package. A wrong entry means a library silently rewritten into a language nobody asked for. The
two failure modes are not symmetric, so the table grows on request rather than on inference.

Open an issue naming the two languages, the direction, whether it holds both ways, and why a
reader of the first can use the second. Two things disqualify a proposal. Anything derivable from
macrolanguage folding, script or region is already tiers 0 to 2 and needs no entry. And a one-way
claim is shared literacy rather than interchangeability, whatever its distance, so it belongs on
the upper tier and inherits that tier's opt-in.

## Contributing

Issues and PRs are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the conventions and how to
run the checks locally.

## Disclaimer

This project is built with care and follows security best practices, but it is intended for personal / self-hosted use. No guarantees of fitness for production environments. Use at your own risk.

This project was built with AI-assisted tooling using [Claude](https://claude.com), [GPT](https://openai.com), and [Kiro](https://kiro.dev). The human maintainer defines architecture, supervises implementation, and makes all final decisions.

## License

Apache-2.0. See [LICENSE](LICENSE).
