// Package subtag holds the CLDR subtag shapes langtag's generated tables are
// built from.
package subtag

import "regexp"

// Language, Region and Script match a lowercase language subtag, an uppercase
// or numeric region subtag and a title-case script subtag.
var (
	Language = regexp.MustCompile(`^[a-z]{2,8}$`)
	Region   = regexp.MustCompile(`^(?:[A-Z]{2}|\d{3})$`)
	Script   = regexp.MustCompile(`^[A-Z][a-z]{3}$`)
)
