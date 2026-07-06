package main

import "time"

/*
nowRFC3339 returns the current time as an RFC3339 string. Isolated here so
the rest of main stays deterministic and easy to reason about.
*/
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
