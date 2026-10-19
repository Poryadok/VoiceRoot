package store

import "errors"

// ReportThreshold is defined only for positive, complete authoritative audience.
// Zero stays a manual-only decision; this function cannot invent a denominator.
func ReportThreshold(audience int64) (int64, error) {
	if audience <= 0 {
		return 0, errors.New("positive authoritative report audience required")
	}
	threshold := audience / 100
	if audience%100 != 0 {
		threshold++
	}
	if threshold < 10 {
		threshold = 10
	}
	return threshold, nil
}
