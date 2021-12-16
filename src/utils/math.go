package utils

import "time"

func MaxDuration(a time.Duration, b time.Duration) time.Duration {
	if a > b {
		return a
	}

	return b
}

func MinDuration(a time.Duration, b time.Duration) time.Duration {
	if a < b {
		return a
	}

	return b
}

func MaxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}

	return b
}

func MinInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}

	return b
}
