# Data sources

This page names the files behind langtag's generated tables, for a developer who reviews a regeneration or checks where an answer comes from.

Every table reads one pin, [`internal/gen/cldr/cldr.go`](../internal/gen/cldr/cldr.go). It holds the CLDR release, the [cldr-json](https://github.com/unicode-org/cldr-json) release packaging it and the SHA-256 of every input file, and every generator reads it, so a build never mixes two CLDR versions. Each generated table's header names the release and the SHA-256 of each file it read, and `TestOnePin` fails when a header names anything else.

| Table | Inputs |
| --- | --- |
| `scripttable.go` | `common/supplemental/likelySubtags.xml` from [CLDR](https://github.com/unicode-org/cldr) |
| `regiontable.go` | `common/supplemental/supplementalData.xml` and `common/supplemental/languageInfo.xml` from CLDR |
| `names/table.go` | `languages.json`, `scripts.json`, `territories.json` and `localeDisplayNames.json` of the English locale in cldr-json, and the ISO 639-2 Registration Authority's [`ISO-639-2_utf-8.txt`](https://www.loc.gov/standards/iso639-2/ISO-639-2_utf-8.txt) |

## Likely scripts

`scripttable.go` holds the script `Tag.Script` reports for a tag that states none. It keeps the rows of `likelySubtags.xml` keyed by a language alone and those keyed by a language and a region. It drops the `und` rows and the rows keyed by a language and a script, because a stated script needs no lookup.

A tag's script is its stated script. Otherwise it is the script of the first row keyed by the tag's language and region, its macrolanguage and region, its language, then its macrolanguage. `cmn-MY` and `zh-MY` agree because the lookup falls back from `cmn` to `zh`. A language with no row, such as `cnr`, reports `Zzzz`.

`golang.org/x/text` carries an older copy of this data that disagrees with CLDR 48. It resolves `zh-MY` to Traditional Chinese, `ku-IQ` to Latin and `ms-ID` to Arabic. langtag never reads it.

## Region containment and clusters

`regiontable.go` lists, for each region, every region group that contains it, and the regions of each cluster. The [regional standards](tiers.md#regional-standards) tier reads it three times: to find a cluster's regions, to place a region inside or outside a cluster, and to leave a pair alone when one region contains the other. The containment comes from the `territoryContainment` element of `supplementalData.xml`, and the clusters from the `matchVariable` entries of `languageInfo.xml` and the `languageMatch` rules naming them.

Every group row counts, including the grouping rows (`419`, `EU`, `UN`) and the deprecated ones (`AN` in `029`). Containment is transitive, so `MX` sits in `013`, `419`, `003`, `019` and `001`.

`golang.org/x/text` carries CLDR 32's containment, which puts `BV`, `GS` and `UM` in `QO`. langtag never reads it.

`matchVariables` in `regiontable.go` lists the regions of each cluster. The generator reads a variable's value as UTS #35 writes it: region codes joined by `+`, a union, and `-`, a set difference, read left to right, a group standing for every region it contains. A group joins the list only when every region it contains is in it, so `019` takes in `419` and `MX`.

`regionClusters` in `regiontable.go` binds each cluster to its language, and to the Traditional script for `$cnsar`, as the `languageMatch` rules do. The generator does not trust the rules' shape. It evaluates them as [UTS #35 language matching](https://www.unicode.org/reports/tr35/tr35.html#EnhancedLanguageMatching) does: the first rule matching a pair of regions gives their distance. It keeps a cluster only when every pair of regions across it is rated farther apart than every pair on one side, over every pair of placed regions where neither contains the other. Otherwise it fails and names the two pairs. CLDR 48 rates a pair on one side 3 or 4 and every pair across 5. The generator also refuses what the table cannot hold: a variable named for every language, two clusters for one language and script, and a variable no rule names.

`TestRegionTableMatchesCopy`, `TestMatchVariablesMatchCopy` and `TestRegionClustersMatchCopy` compare the table with hand copies of the rows and clusters kept in `regions_test.go`, each citing the release and lines it was copied from. `TestMultiScriptMatchesCopy` in `scripttable_test.go` and `TestResolvedScript` in `scripts_test.go` do the same for the script table. A hand edit or a regeneration that moves a row these tests copy fails them until a reviewer updates the copies. Any other hand edit shows as a diff when the tables are regenerated from the pin.

## English names

`names/table.go` holds the names `names.English` composes. The language, script and region names and the display pattern, `{0} ({1})` with a comma between a script and a region, come from CLDR's English locale in cldr-json. The table keeps the entries keyed by one subtag and drops the combined ones, so `pt-BR` is "Portuguese (Brazil)" and never "Brazilian Portuguese". Scripts take their in-context names ("Simplified") and regions their long names ("Hong Kong SAR China").

A language CLDR's English locale leaves unnamed takes the first English name the ISO 639-2 Registration Authority gives it. CLDR 48 names no `cnr`, so Montenegrin comes from that file. Where `Parse` turns a listed code into a subtag with no name of its own, that subtag takes the code's name: ISO 639-2 lists `bih`, which `Parse` reads as `bh`, so `bh` is "Bihari languages" too.

`names/testdata/names.golden` lists every row and a set of composed names, and `TestNamesGolden` fails until it matches the compiled table.

## Regenerating

An ordinary run downloads every pinned file, refuses any whose SHA-256 differs from the pin, and rewrites each table:

```sh
go generate ./...
```

To move to a newer CLDR release, first report how the script table changes, then move the pin and regenerate. The pin command fetches every CLDR and cldr-json file at the new release, so it fails when cldr-json has not published it, and refuses a cldr-json release that packages another CLDR release (`release-<major>-<minor>` is cldr-json `<major>.<minor>.x`):

```sh
go run ./internal/gen/likely -diff <tag> > likely-diff.md
go run ./internal/gen/pin -release <tag> -json <cldr-json release>
go generate ./...
UPDATE_GOLDEN=1 go test -run TestNamesGolden ./names
```

The report compares the rows of the pinned `likelySubtags.xml` with those of `<tag>`, in memory, and writes no file. It names the languages whose `Tag.MultiScript` answer changes, including those that change through their macrolanguage (`kmr` through `ku`), and every row added, removed or given another script. An unchanged table writes nothing. Errors go to standard error. The pull request then shows every script, region and name change as a diff of the generated tables to review.

The ISO 639-2 file has no versions, so moving the CLDR pin keeps its SHA-256. When the Registration Authority edits the file, the names fail to regenerate until the pin moves with today's fetch date:

```sh
go run ./internal/gen/pin -iso
```

### The monthly refresh

One command runs the steps above for the newest release:

```sh
go run ./internal/gen/refresh
```

It reads the latest CLDR release tag and the newest cldr-json release of that CLDR release, and fails when cldr-json has not published it yet. `-release <tag>` names another CLDR release. It prints `no change` when the pin already names both. Otherwise it writes the script table's report, moves the pin, regenerates every table, updates the names golden and prints each path it changed. With `-out <dir>` it also writes a proposal there: `proposal.json` lists the paths it changed, none when the pin did not move, `files/` holds their new content, and `title` and `body.md`, the report included, hold the pull request's text. `GITHUB_TOKEN`, when set, authenticates its GitHub API reads. A failed regeneration names the ISO 639-2 pin above, which a refresh never moves.

The `community-data-refresh.yaml` workflow runs this command on `dev` on the 3rd of each month, and on demand with an optional `release`. It holds no secret and uploads the proposal as an artifact. The [community data refresh](https://github.com/cplieger/ci/blob/main/docs/workflows.md#workflows-for-this-repository-only) in cplieger/ci reads that artifact and, when the proposal changes anything, opens or updates the `community-data-refresh` pull request against `dev`. The pull request needs a passing `ci / validate` and a human merge.
