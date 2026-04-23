package pricing

import "fmt"

const (
	// MicroCentsPerDollar is the number of microcents in one US dollar.
	MicroCentsPerDollar int64 = 100_000

	// MicroCentsPerCent is the number of microcents in one US cent.
	MicroCentsPerCent int64 = 1_000
)

// DollarToMicrocents converts a float64 dollar amount received from
// a pricing API into an int64 microcent value. This is the only place
// in the pricing package where float64 currency arithmetic occurs.
//
// Examples:
//
//	0.50    ->  50,000
//	1.00    -> 100,000
//	0.0034  ->     340
//	0.12345 ->  12,345
func DollarToMicrocents(dollars float64) int64 {
	return int64(dollars * float64(MicroCentsPerDollar))
}

// MicrocentsToDisplay renders a microcent value as a human-readable
// dollar string with two decimal places.
//
// Examples:
//
//	 50,000 -> "$0.50"
//	100,000 -> "$1.00"
//	123,456 -> "$1.23"
//	    340 -> "$0.00"  (sub-cent values round down at display time)
func MicrocentsToDisplay(mc int64) string {
	dollars := mc / MicroCentsPerDollar
	cents := (mc % MicroCentsPerDollar) / MicroCentsPerCent
	return fmt.Sprintf("$%d.%02d", dollars, cents)
}

// MicrocentsToDetailedDisplay renders a microcent value with full
// precision for use in cost breakdowns where sub-cent accuracy matters.
//
// Examples:
//
//	 50,000 -> "$0.50000"
//	123,456 -> "$1.23456"
func MicrocentsToDetailedDisplay(mc int64) string {
	dollars := mc / MicroCentsPerDollar
	remainder := mc % MicroCentsPerDollar
	return fmt.Sprintf("$%d.%05d", dollars, remainder)
}
