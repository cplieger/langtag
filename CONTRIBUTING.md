# Contributing to langtag

The [shared rules](https://github.com/cplieger/.github/blob/main/CONTRIBUTING.md) for commits, releases, synced files and checks apply here.

## Rules

- After adding a `builtinFallbacks` entry in `fallbacks.go`, update its table and total in [docs/fallback-table.md](docs/fallback-table.md). Recalculate the README counts. Update [docs/non-goals.md](docs/non-goals.md) for a shared-literacy entry. If its CLDR distance extends the range in `docs/tiers.md`, update it. No test checks these pages.
- A new pair in `closeScripts` in `scripts.go` also goes into the table under [Close scripts](docs/tiers.md#close-scripts) in `docs/tiers.md`, which states how many pairs remain. No test checks that page either.

## Checks

Before you change `Parse` in `tag.go`, fuzz it locally:

```sh
go test -run '^$' -fuzz FuzzParse -fuzztime 60s
```

Pull-request CI runs `go test`, which replays only the seeds in `FuzzParse` and the inputs committed under `testdata/fuzz/`, so it never generates new input. Commit any failing input the run writes to `testdata/fuzz/FuzzParse/` with the fix.
