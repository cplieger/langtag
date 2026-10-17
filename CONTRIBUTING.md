# Contributing to langtag

The [shared rules](https://github.com/cplieger/.github/blob/main/CONTRIBUTING.md) for commits, releases, synced files and checks apply here.

## Rules

- After adding a `builtinFallbacks` entry in `fallbacks.go`, update its table and total in [docs/fallback-table.md](docs/fallback-table.md). Recalculate the README counts. Update [docs/non-goals.md](docs/non-goals.md) for a shared-literacy entry. If its CLDR distance extends the range in `docs/tiers.md`, update it. No test checks these pages.
- A new pair in `closeScripts` in `scripts.go` also goes into the table under [Close scripts](docs/tiers.md#close-scripts) in `docs/tiers.md`, which states how many pairs remain. No test checks that page either.
- `scripttable.go`, `regiontable.go`, `names/table.go` and the pin `internal/gen/cldr/cldr.go` are generated: never edit them by hand. Move the pin, regenerate and update `names/testdata/names.golden` as [Data sources](docs/data-sources.md#regenerating) describes. `go run ./internal/gen/refresh` runs those commands for the newest CLDR release, and the monthly `community-data-refresh` pull request is its output. Review the regenerated tables and the golden as a diff, then update the hand copies in `regions_test.go`, `scripttable_test.go` and `scripts_test.go` that the change turns red, with the release and lines each cites.
- A change to a cluster's regions or to which language it binds also goes into the table under [Regional standards](docs/tiers.md#regional-standards) in `docs/tiers.md`, with its `languageInfo.xml` line provenance. A script or region example, or a CLDR release or line citation, the change makes false in `README.md`, `docs/parsing.md`, `docs/tiers.md` or `docs/data-sources.md` is corrected in the same pull request. No test checks these pages.

## Checks

Before you change `Parse` in `tag.go`, fuzz it locally:

```sh
go test -run '^$' -fuzz FuzzParse -fuzztime 60s
```

Pull-request CI runs `go test`, which replays only the seeds in `FuzzParse` and the inputs committed under `testdata/fuzz/`, so it never generates new input. Commit any failing input the run writes to `testdata/fuzz/FuzzParse/` with the fix.
