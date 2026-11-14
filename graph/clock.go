package graph

import "time"

// now reports the current time. It is a package variable so a test can freeze
// it and compare solver timestamps deterministically.
var now = func() time.Time { return time.Now() }
