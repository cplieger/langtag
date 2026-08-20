# langtag

[![Go Reference](https://pkg.go.dev/badge/github.com/cplieger/langtag/v2.svg)](https://pkg.go.dev/github.com/cplieger/langtag/v2)
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

`go get github.com/cplieger/langtag/v2@latest`

## Usage

```go
package main

import (
	"fmt"

	"github.com/cplieger/langtag/v2"
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

## Migrating from v1

v2 moves the wanted language out of the argument lists and into a constructed
`Preference`, so the direction of a comparison is fixed where the preference is
built rather than re-stated, swappably, at every call site.

Every import path gains the `/v2` suffix, because v2 is a new Go module path. Four renames follow:

| v1 | v2 |
| --- | --- |
| `langtag.Compare(want, have)` | `langtag.Prefer(want).Compare(have)` |
| `langtag.Reason(want, have)` | `langtag.Prefer(want).Reason(have)` |
| `langtag.Match(want, have, floor)` | `langtag.Prefer(want).Match(have, floor)` |
| `langtag.Best(want, candidates, tagOf, floor)` | `langtag.Prefer(want).Best(candidates, tagOf, floor)` |

A custom table binds the same way: `c.Compare(want, have)` and friends become
`c.Prefer(want).Compare(have)`, and `BestWith(c, want, ...)` becomes
`c.Prefer(want).Best(...)`. Grading is unchanged: every pair lands on the same
tier it did in v1.

All four are methods on `Preference`. `Best` is a generic method, which Go
gained in 1.27, so v2 requires that toolchain; it also means `Best` cannot
appear in an interface, so a caller abstracting over selection wraps it in a
non-generic method of its own.

## API

- **Parsing**: `Parse`, `MustParse` and `Valid` take a raw identifier. `Tag` reports `String`,
  `Language`, `Script` and `IsZero`.
- **Preferences**: `Prefer` binds the wanted language to the built-in table, and `Comparer.Prefer`
  binds it to a custom one. `Preference` carries `Compare`, `Reason`, `Match`, `Best`, `Want` and
  `String`.
- **Tiers**: six `Tier` constants, with `Tier.String` and `ParseTier` for the configuration
  spellings.
- **Tables**: `Fallbacks` and `CloseScripts` return copies of the shipped judgments.
  `WithFallbacks` replaces the cross-language table, `ValidateFallbacks` names the entries it
  drops, and `Default` returns the built-in `Comparer`. `Fallback`, `CloseScript` and `Kind` are
  the table types.

## Parsing

`Parse` accepts ISO 639-1, ISO 639-2 in either the bibliographic or terminological variant,
ISO 639-3, and BCP 47 tags, in any ASCII letter case, with surrounding whitespace. It returns
`ok == false` rather than an error, because ignoring the tag is the only available recovery.

The accept set is ASCII alphanumerics only, because [RFC 5646 §2.1](https://www.rfc-editor.org/rfc/rfc5646#section-2.1)
defines every subtag that way. Case folding is therefore ASCII byte arithmetic rather than a
Unicode fold, which has two consequences worth relying on. `Parse`'s answers do not move when the
toolchain's Unicode version does; verified across Unicode 15 and 17, over which the accept set and
every canonical form are byte-identical while `strings.EqualFold` changed its answer for three rune
pairs. And a rune that a Unicode fold maps onto ASCII cannot impersonate a subtag: `İd` is not
Indonesian and `ſk` is not Slovak, though a Unicode-aware comparison would call both equal to the
real thing.

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

`Prefer(want)` binds the language a person chose, and `p.Compare(have)` reports how far the
available language sits from it. A caller passes the highest tier it will accept as a floor.

The order is by **kind of license**, from narrowest to widest, not by reading difficulty. Reading
difficulty is not monotonic along it, and `TierOtherScript` is where that bites. CLDR scores a
generic same-language cross-script substitution at 50, farther than every pair on the two tiers
above it, so accepting `other-script` is a bigger ask for Chinese content than accepting
`intelligible` is for Norwegian.

The script pairs CLDR _does_ explicitly rate as close sit on the tier below instead, so what remains
on `other-script` is the set CLDR does not vouch for. See [Close scripts](#close-scripts).

## Close scripts

A script difference is normally farther than a region difference. A few languages are written in
two scripts whose readers are taught both, where it is no barrier at all, and those sit at
`TierSameLanguage`.

Membership is derived rather than judged. CLDR names only a handful of same-language script pairs
against its generic 50, and all but one are **one-way** transliteration rows (`ja-Latn` onto
`ja-Jpan`, `hi-Latn` onto `hi-Deva`, and the other Latin romanizations). A romanization feeding its
native script is not two audiences reading each other, which is the same reason a one-way language
relation is shared literacy rather than interchangeability. One symmetric pair remains:

| Language | Scripts | Why | Provenance |
| --- | --- | --- | --- |
| `sr` | `Latn` and `Cyrl` | Serbian is written in both and Serbian schooling teaches both | CLDR distance 5, symmetric, against a generic 50 |

`CloseScripts` returns the list. Unlike the cross-language table it is not replaceable, because it
records where CLDR states an explicit symmetric distance rather than a judgment about people. A
caller that wants a script difference treated as a barrier regardless floors at `same-language`.

Deliberately absent: Simplified against Traditional Chinese, which CLDR does not name and which
therefore sits at the generic 50; and `uz-Latn` against `uz-Cyrl` and `az-Latn` against `az-Cyrl`,
which `x/text`'s stale snapshot rates as close but upstream CLDR does not name either.

| Tier | Meaning | Examples |
| --- | --- | --- |
| `TierIdentical` | The same language, the same canonical tag | `ger` and `deu`, `nor` and `no`, `iw` and `he` |
| `TierSameLanguage` | One language a reader takes in without effort | `nob` and `nor`, `cmn` and `zh`, `es-ES` and `es-419`, `sr-Latn` and `sr-Cyrl` |
| `TierOtherScript` | One language in a script its readers are not taught alongside | `zh-Hans` and `zh-Hant`, `uz-Latn` and `uz-Cyrl` |
| `TierIntelligible` | Two different languages that are close enough to read across | `nb` and `nn`, `no` and `da`, `hr` and `bs`, `cs` and `sk` |
| `TierSharedLiteracy` | A different language readers use because they are broadly literate in it | `ca` to `es` |
| `TierNone` | No relationship worth acting on | `no` and `sv`, `hi` and `ur`, `pl` and `cs`, `gl` and `es` |

The first three tiers follow from published standards data and hold no opinion. The two above
them are curated judgments, and they are separate tiers because they make different claims.
`TierIntelligible` says two languages are close enough that readers move between them, which is a
symmetric property of the languages. `TierSharedLiteracy` says only that readers of one are, as a
population, literate in the other, which is a one-way fact about people and licenses a
substitution to a possibly unrelated language. Accepting the first does not accept the second.

Comparison is **not** commutative. Cross-language relationships are directed, and the direction is
carried by the `Preference` role: `Prefer` names the language a person chose, so
`p.Compare(have)` reads as the person's choice judging the offer, and no call site holds two tags
whose order could be silently swapped. A Catalan viewer accepts a Spanish track; a Spanish viewer
does not accept a Catalan one.

## The cross-language table

Five shipped relationships, each a claim about people rather than about code, each one arguable.
`Fallbacks` returns the table and `Preference.Reason` returns the justification for a given pair,
so a surprised user can be shown why a substitution happened.

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
disables both cross-language tiers. A malformed entry is dropped rather than accommodated, so a
mistake in a table removes a substitution instead of licensing an unintended one, and
`ValidateFallbacks` reports what was dropped and why. When two entries claim one ordered pair at
different tiers, the farther tier wins, so table order cannot decide how close a pair is.

`TierNone` is not a usable floor. It means "no relationship", so accepting it would accept every
language as a substitute for every other. `Match` and `Best` therefore report nothing for a floor at
`TierNone` or beyond, which matters because that is also what `ParseTier` returns for a value it
does not recognise: a mistyped configuration setting matches nothing rather than everything.

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
commensurable). Display names are not shipped: the CLDR tables behind
[`x/text/language/display`](https://pkg.go.dev/golang.org/x/text/language/display) add roughly
2.5 MB to a binary, so a caller that wants them imports that package directly.

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
