# langtag

[![Go Reference](https://pkg.go.dev/badge/github.com/cplieger/langtag.svg)](https://pkg.go.dev/github.com/cplieger/langtag)
[![Go version](https://img.shields.io/github/go-mod/go-version/cplieger/langtag)](https://github.com/cplieger/langtag/blob/main/go.mod)
[![Test coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/cplieger/langtag/badges/coverage.json)](https://github.com/cplieger/langtag/actions/workflows/coverage.yml)
[![Mutation](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/cplieger/langtag/badges/mutation.json)](https://github.com/cplieger/langtag/issues?q=label%3Agremlins-tracker)
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
| `TierSensitive` | A different language a reader can probably use | `nb` and `nn`, `no` and `da`, `hr` and `bs`, `ca` to `es` |
| `TierNone` | No relationship worth acting on | `no` and `sv`, `hi` and `ur`, `cs` and `sk` |

The first three tiers follow from published standards data and hold no opinion. `TierSensitive`
is the only tier that encodes a judgment, which is why it is named for that and why a caller must
ask for it.

`Compare` is **not** commutative. Tier-3 relationships are directed, so the argument order
matters: `want` is the language a person chose, `have` is what the content offers. A Catalan
viewer accepts a Spanish track; a Spanish viewer does not accept a Catalan one.

## The sensitive table

Four shipped relationships, each a claim about people rather than about code, each one arguable.
`Fallbacks` returns the table and `Reason` returns the justification for a given pair, so a
surprised user can be shown why a substitution happened.

| Wanted | Accepted | Both ways | Why |
| --- | --- | --- | --- |
| `no` | `nn` | yes | Bokmål and Nynorsk are the two written standards of Norwegian; Norwegian schooling teaches both |
| `no` | `da` | yes | Written Bokmål derives from Danish and the two remain close in writing |
| `hr` | `bs` | yes | Croatian and Bosnian are mutually intelligible and share the Latin script |
| `ca` | `es` | no | Catalonia is officially bilingual and Spanish is compulsory in schooling |

Entries name languages the way `Tag.Language` reports them, so one entry covers every spelling:
a wanted language of `no` answers for `nor`, `nob`, `no` and `nb` alike.

`WithFallbacks` replaces the table for a deployment that disagrees, and `WithFallbacks(nil)`
disables tier 3 entirely.

## What this package will not do

**It will not substitute an unrelated language because a population reads both.** CLDR's distance
data rates Basque against Spanish, Welsh against English, Tamil against English and Belarusian
against Russian as close, because those populations are broadly literate in the second language.
That is accurate sociolinguistics and a poor substitution rule: a viewer who chose Tamil
subtitles did not ask for English ones.

The exclusion is structural rather than filtered. Tiers 0 through 2 cannot express a
cross-language relationship at all, and the sensitive table is curated by hand, so no amount of
loosening the floor can reach that family. A test enumerates 33 such pairs and asserts every one
of them stays at `TierNone`.

Also absent, deliberately: collation, formatting, content negotiation, `Accept-Language` parsing,
and any numeric distance between tiers. Display names live in the optional `langtag/name`
subpackage, kept separate because its CLDR tables add roughly 2.5 MB to a binary.

## Adding to the table

A missing entry means a track is left alone, which is what a caller had before adopting this
package. A wrong entry means a library silently rewritten into a language nobody asked for. The
two failure modes are not symmetric, so the table grows on request rather than on inference.

Open an issue naming the two languages, the direction, and why a reader of the first can use the
second. Anything derivable from macrolanguage folding, script or region is already tiers 0 to 2
and needs no entry.

## Contributing

Issues and PRs are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the conventions and how to
run the checks locally.

## Disclaimer

This project is built with care and follows security best practices, but it is intended for personal / self-hosted use. No guarantees of fitness for production environments. Use at your own risk.

This project was built with AI-assisted tooling using [Claude](https://claude.com), [GPT](https://openai.com), and [Kiro](https://kiro.dev). The human maintainer defines architecture, supervises implementation, and makes all final decisions.

## License

Apache-2.0. See [LICENSE](LICENSE).
