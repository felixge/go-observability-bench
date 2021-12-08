package internal

import "time"

func TruncateDuration(d time.Duration) time.Duration {
	magnitude := time.Duration(1)
	for {
		if magnitude > d {
			return d.Truncate(magnitude / 1000)
		}
		magnitude = magnitude * 10
	}
}
