# Parsing language tags

This page covers what `Parse` accepts, how it canonicalizes a tag and what it rejects, for a developer who feeds it identifiers from media files, transcoders or third-party APIs.

## What Parse accepts

`Parse` accepts ISO 639-1, ISO 639-2 in its bibliographic or terminological form, ISO 639-3 and BCP 47 tags. It ignores ASCII letter case and surrounding whitespace. It returns `ok == false` instead of an error, because the only recovery from a malformed tag is to ignore it.

`Valid` gives the same answer as a bool. Use it to check configuration when it loads, so a typo fails at startup. `MustParse` panics on input that names no language, so it belongs in package-level tables and tests, never on a path that handles input.

## ASCII only

[RFC 5646 §2.1](https://www.rfc-editor.org/rfc/rfc5646#section-2.1) defines every subtag as ASCII letters and digits. `Parse` accepts only those and folds letter case with ASCII arithmetic, never with a Unicode fold. That has two effects you can rely on:

- `Parse` gives the same answers when the Go toolchain moves to a new Unicode version. Its accept set and every canonical form measured byte-identical on Unicode 15 and 17, while `strings.EqualFold` changed its answer for three rune pairs.
- A rune that a Unicode fold maps onto ASCII cannot pose as a subtag. `İd` is not Indonesian and `ſk` is not Slovak, though a Unicode-aware comparison calls both equal to the real code.

## Canonicalization

`Parse` maps the code systems that spell one language several ways onto one BCP 47 tag:

| Input | Canonical | Language |
| --- | --- | --- |
| `ger`, `deu`, `de` | `de` | `de` |
| `chi`, `zho`, `zh` | `zh` | `zh` |
| `iw` | `he` | `he` |
| `nor`, `no` | `no` | `no` |
| `nob`, `nb` | `nb` | `no` |
| `cmn` | `cmn` | `zh` |

`Tag.Language` reports the primary subtag after macrolanguage folding. A macrolanguage is an umbrella code, such as `no` for Norwegian, and folding maps its dominant variety onto it. So `nob` and `nor` share the language `no` while keeping distinct tags. Use `Language` as the key for a cache or a learned-preference map, because it does not split across spellings.

Only the dominant variety folds. `nob` folds onto `no` and `cmn` onto `zh`, while `nno` for Nynorsk and `yue` for Cantonese keep their own language.

`Tag.Script` reports the ISO 15924 script. When a tag states none, it reports the default that [CLDR](https://cldr.unicode.org), the Unicode Consortium's shared locale data, gives the language, so a bare `zh` compares as `zh-Hans` and a bare `sr` as `sr-Cyrl`.

## What Parse rejects

`Parse` returns `ok == false` and the zero `Tag` for these inputs:

- the empty string
- the placeholder subtags that name no language, which are `und`, `zxx`, `mul` and `mis`
- private-use subtags, `qaa` through `qtz`
- anything the IANA Language Subtag Registry does not know

`Parse` discards variant and extension subtags, such as `-u-`, `-t-`, `-x-` and `ca-valencia`, because none of them changes which language a track is in. It builds the tag from the language, script and region only, so the canonical form survives a round trip and works as a stored key.

## The zero Tag

The zero `Tag` matches nothing at any tier, including another zero `Tag`. Two tracks both labelled undetermined are not known to share a language.
