# The cross-language table

This page lists the five relationships between different languages that langtag ships. It shows how to replace them and how to propose a new one, for a developer who wants to change what the curated tiers accept.

## The shipped entries

Each entry is a claim about people, and each one is arguable. `Fallbacks` returns the table, and `Preference.Reason` returns the reason for a given pair, so a surprised user can be shown why a substitution happened.

| Wanted | Accepted | Kind | Both ways | Why |
| --- | --- | --- | --- | --- |
| `no` | `nn` | intelligible | yes | Bokmål and Nynorsk are the two written standards of Norwegian. Norwegian schooling teaches both |
| `no` | `da` | intelligible | yes | Written Bokmål derives from Danish and the two remain close on the page |
| `hr` | `bs` | intelligible | yes | Croatian and Bosnian are mutually intelligible and share the Latin script |
| `cs` | `sk` | intelligible | yes | Czech and Slovak are mutually intelligible in writing |
| `ca` | `es` | shared-literacy | no | Catalonia is officially bilingual and Spanish is compulsory in schooling |

Entries name languages the way `Tag.Language` reports them, so one entry covers every spelling. A wanted language of `no` answers for `nor`, `nob`, `no` and `nb` alike.

The `Kind` decides the tier, and it must agree with the direction. An `Intelligible` entry runs both ways, because that is what the claim means. A `SharedLiteracy` entry runs one way, because a majority-language population does not read the minority language back. `WithFallbacks` enforces both rules on every table, and a test checks the shipped one.

Every entry carries a `Provenance` field naming where the claim can be checked again. For all five it is the [`languageInfo.xml`](https://github.com/unicode-org/cldr/blob/main/common/supplemental/languageInfo.xml) file of [CLDR](https://cldr.unicode.org), the Unicode Consortium's shared locale data.

## Replacing the table

`WithFallbacks` builds a `Comparer` from your own table, and `Comparer.Prefer` binds a wanted language to it. `WithFallbacks(nil)` turns off both cross-language tiers.

A malformed entry is dropped, so a mistake removes a substitution instead of allowing one you did not intend. An entry is malformed when either side is empty, both sides name one language, its `Kind` is unset or unknown, or its `Kind` disagrees with its direction. `ValidateFallbacks` reports each dropped entry and why.

When two entries claim one ordered pair at different tiers, the farther tier wins. So the order of the table cannot decide how close a pair is. `ValidateFallbacks` reports that conflict too.

## Checking a claim against CLDR

`golang.org/x/text` embeds CLDR version 32, which disagrees with current CLDR. Its matcher does not know `cs` and `sk` or `ca` and `es`. It still carries a `mk` to `bg` relationship that CLDR has since retired. To check whether CLDR carries a relationship, its direction and its distance, read `languageInfo.xml` itself, never the `x/text` matcher.

langtag does not use the `x/text` Matcher. It uses `x/text` to parse and canonicalize tags, and to infer a default script when a tag states none. That default comes from the same CLDR 32 snapshot.

## Proposing an entry

A missing entry leaves a track alone, which is what a caller had before adopting langtag. A wrong entry silently switches a library's tracks to a language nobody asked for. The two failures are not equal, so the table grows on request rather than on inference.

Open an issue that names the two languages, the direction, whether it holds both ways, and why a reader of the first can use the second. Two things rule a proposal out. Anything that follows from [macrolanguage folding](parsing.md#canonicalization), script or region is already covered by the first three tiers and needs no entry. A one-way claim is shared literacy, whatever its distance. It goes on `TierSharedLiteracy` and needs that tier's opt-in.
