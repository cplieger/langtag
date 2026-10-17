//go:build race

package langtag_test

// raceEnabled reports a -race build, where TestParseAllocations checks each
// case's raceSlack ceiling instead of its exact pin.
const raceEnabled = true
