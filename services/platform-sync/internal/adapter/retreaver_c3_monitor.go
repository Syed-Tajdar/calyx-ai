package adapter

import (
	"log/slog"
	"sync/atomic"
)

// Stage 4 R1-fix F3 — watch the C3 assumption rather than trusting it.
//
// C3 was resolved without a migration on the strength of a 100-call sample in which
// `revenue = null` held for exactly the 60 unconverted calls and was populated on exactly the 40
// converted ones — never once `0`. That makes `null → 0.00` lossless *while the correlation
// holds*, because `converted` is mapped alongside the amount and disambiguates it.
//
// It is an empirical correlation, not a documented API guarantee. If Retreaver ever reports a
// converted call with no revenue, or revenue on an unconverted call, the stored 0.00 stops being
// unambiguous and the owner revisits the Nullable(Decimal) migration. These counters and log
// lines are what turn that from a silent wrong number into something visible in R2.
//
// Rule 5: only the call UUID prefix is ever logged — never the caller, never credentials.

// RetreaverC3Counters tallies calls that break the C3 correlation.
type RetreaverC3Counters struct {
	Checked                  atomic.Uint64
	ConvertedWithoutRevenue  atomic.Uint64
	RevenueWithoutConversion atomic.Uint64
	ConvertedZeroRevenue     atomic.Uint64
}

var retreaverC3 RetreaverC3Counters

// RetreaverC3Snapshot reads the counters. Reported at the end of a sync run and in R2.
func RetreaverC3Snapshot() (checked, convertedWithoutRevenue, revenueWithoutConversion, convertedZeroRevenue uint64) {
	return retreaverC3.Checked.Load(),
		retreaverC3.ConvertedWithoutRevenue.Load(),
		retreaverC3.RevenueWithoutConversion.Load(),
		retreaverC3.ConvertedZeroRevenue.Load()
}

// ResetRetreaverC3 zeroes the counters. For tests.
func ResetRetreaverC3() {
	retreaverC3.Checked.Store(0)
	retreaverC3.ConvertedWithoutRevenue.Store(0)
	retreaverC3.RevenueWithoutConversion.Store(0)
	retreaverC3.ConvertedZeroRevenue.Store(0)
}

// callUUIDPrefix shortens a call UUID for logging. Enough to find the call in the Retreaver UI,
// not enough to be a useful identifier anywhere else.
func callUUIDPrefix(uuid string) string {
	if len(uuid) > 8 {
		return uuid[:8]
	}
	return uuid
}

// recordRetreaverC3 checks one mapped call against the C3 correlation.
//
// `converted` and `revenue` are the raw pointers from the API payload, so a missing key is
// distinguishable from a false/zero value — which is the whole point.
func recordRetreaverC3(uuid string, converted *bool, revenue *float64) {
	retreaverC3.Checked.Add(1)

	isConverted := converted != nil && *converted

	switch {
	case isConverted && revenue == nil:
		retreaverC3.ConvertedWithoutRevenue.Add(1)
		slog.Warn("retreaver C3 violation: converted call reports no revenue",
			"call", callUUIDPrefix(uuid),
			"rule", "converted=true implies revenue is present",
			"impact", "revenue stored as 0.00 is no longer distinguishable from 'not reported'")

	case !isConverted && revenue != nil:
		retreaverC3.RevenueWithoutConversion.Add(1)
		slog.Warn("retreaver C3 violation: unconverted call reports revenue",
			"call", callUUIDPrefix(uuid),
			"rule", "revenue present implies converted=true",
			"impact", "an unconverted call carries money; the C3 correlation no longer holds")

	case isConverted && revenue != nil && *revenue == 0:
		// The one case the null → 0.00 mapping genuinely cannot express: a real $0 conversion
		// is stored identically to "not converted, no money".
		retreaverC3.ConvertedZeroRevenue.Add(1)
		slog.Warn("retreaver C3 violation: converted call reports exactly zero revenue",
			"call", callUUIDPrefix(uuid),
			"rule", "converted=true implies revenue != 0",
			"impact", "a genuine $0 conversion is indistinguishable from an unconverted call")
	}
}
