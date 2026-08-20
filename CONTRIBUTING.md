# Contributing to langtag

Thanks for taking a look. This file covers what the library is for, what
belongs in it, and how to run the checks locally.

## What the library is

`langtag` answers one question: may this language track stand in for the one
that was asked for, and how far a substitution is that. It canonicalizes
language identifiers from any published code system onto one BCP 47 form, then
grades the distance between wanted and available on a five-step tier scale.

The only runtime dependency is
[`golang.org/x/text`](https://pkg.go.dev/golang.org/x/text), used for parsing
and canonicalization. That is the part of the problem with objectively correct
answers, backed by the IANA Language Subtag Registry and over two hundred
mappings nobody should maintain by hand.

The tier policy is the library's own. It deliberately does **not** use
`x/text`'s CLDR matcher, for reasons worth knowing before proposing a change:

- The matcher's four confidence levels fuse two unrelated ideas. `Exact`
  contains 32 pairs whose canonical tags differ, and both `High` and `Low`
  carry CLDR's written-language fallback data, which records that a population
  is broadly literate in a second language. Basque against Spanish, Welsh
  against English and Tamil against English all rate as close. That is accurate
  sociolinguistics and a poor substitution rule.
- Its buckets cannot express `TierIdentical`: identity and macrolanguage
  equivalence both land in `Exact`.
- It carries no relation between Catalan and Spanish at any confidence, so the
  shipped table is needed regardless, and two overlapping policy sources are
  worse than one.
- It is opaque. "Why did it pick Danish" has no answer a user can read, where
  a table entry carries its own justification.

`TestCompareExcludesSharedLiteracy` pins the exclusion over 33 pairs. If a
change makes it fail, that is the design being violated rather than a test
needing an update.

## Scope

In scope: canonicalization, the tier scale, the sensitive-fallback table, and
display names in the `langtag/name` subpackage.

Out of scope, deliberately: collation, formatting, content negotiation,
`Accept-Language` parsing, and a numeric distance between tiers (a number
invites arithmetic across steps that are not commensurable).

### Proposing a table entry

The sensitive table is the only place the library holds an opinion. A missing
entry means a track is left alone, which is what a caller had before adopting
the package. A wrong entry means a library silently rewritten into a language
nobody asked for. Those failure modes are not symmetric, so the table grows on
request rather than on inference.

Open an issue with:

- the two languages, as `Tag.Language` reports them (`no`, not `nb` or `nor`);
- the direction, and whether it holds both ways;
- why a reader of the first can use the second.

Two things disqualify a proposal. Anything derivable from macrolanguage
folding, script or region is already tiers 0 to 2 and needs no entry. And
shared literacy is not interchangeability: that a population reads a majority
language does not make the two languages substitutes.

## Local development

The module targets the Go version pinned in `go.mod`. Use that toolchain or
newer.

```sh
go build ./...
go test ./...
go test -race ./...
```

### Linting and formatting

Lint config lives in `.golangci.yaml` (golangci-lint v2). Formatting is
`gofumpt` (`extra-rules`) plus `gci` import grouping; `golangci-lint run`
reports unformatted files as issues, so format before pushing.

```sh
golangci-lint run
golangci-lint fmt
```

### Fuzzing

`Parse` is the untrusted-input boundary: identifiers reach it from media
metadata, transcoder output and third-party APIs. `FuzzParse` asserts more than
absence of panics. A rejected tag must be the zero value and must match
nothing; an accepted tag must name a language, must be a fixed point so that its
canonical form survives a round trip through storage, and must be ASCII, because
the canonical form is a persistence key and a non-ASCII byte in one would expose
a consumer's own comparison to the Unicode fold tables this package never
consults.

```sh
go test -run '^$' -fuzz FuzzParse -fuzztime 60s
```

The committed corpus under `testdata/fuzz/` holds inputs that once failed and
runs on every `go test`. `AA-u-0A-0A-u-00-00` is there because extension
subtags made the canonical form non-idempotent, which is why `Parse` now
composes the tag from exactly language, script and region.

`FuzzParse` also seeds one input per class of Unicode 15-to-17 change: a newly
folding pair, each category flip, a rune that folds or lowercases onto ASCII, a
newly gained uppercase mapping, and a newly assigned letter. Coverage-guided
fuzzing is unlikely to construct those runes on its own, and the weekly corpus
does not persist between runs, so the committed seeds are the durable coverage.
`unicode_test.go` asserts the same boundary exhaustively and explains each class.

### Mutation testing

`.gremlins.yaml` configures [Gremlins](https://gremlins.dev) mutation testing
(synced from `cplieger/ci`; change it upstream). Run it locally to confirm new
tests actually kill mutants:

```sh
gremlins unleash .
```

## Test suite conventions

Tests are black-box (`package langtag_test`) and exercise the public API only.

- `tag_test.go`: parsing, canonicalization across code systems, rejection of
  placeholder and private-use subtags, and `FuzzParse`.
- `compare_test.go`: the tier table case by case, the shared-literacy
  exclusion, the directedness contract, and the monotonicity property that
  makes the tiers a scale rather than a set of labels.
- `best_test.go`: candidate selection, table overrides, and the configuration
  surface.
- `kind_test.go`: the Kind-versus-direction rule, table validation, and the
  fail-closed contracts (an unusable floor, a zero `Preference`, a nil table).
- `scripts_test.go`: close-script membership and the provenance of each entry.
- `unicode_test.go`: the ASCII accept set, asserted rune by rune against the
  Unicode 15 and 17 fold tables.
- `example_test.go`: runnable examples, which are verified documentation.

Table-driven cases live in maps so iteration order is randomized, and every
case must pass when run alone. Failure messages name the function, its inputs,
what it returned and what was wanted, in that order, so a failure is
diagnosable without opening the test file.

## Commits and PRs

Branch from `main`, keep changes focused with tests, and open a PR. This
account uses [Conventional Commits](https://www.conventionalcommits.org/)
parsed by git-cliff (`cliff.toml`), so the commit type drives the version
bump: `feat:`, `fix:`, `sec:`, and `chore:`/`docs:`/`refactor:`/`test:` (no
release). Write the subject as the changelog line a consumer would read.

## Conduct & security

By participating you agree to the org-wide
[Code of Conduct](https://github.com/cplieger/.github/blob/main/CODE_OF_CONDUCT.md).
Report security issues through the
[security policy](https://github.com/cplieger/.github/blob/main/SECURITY.md),
never in a public issue.
