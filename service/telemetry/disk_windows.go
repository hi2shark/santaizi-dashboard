//go:build windows

package telemetry

func volumeFreeBytes(string) (uint64, bool) {
	return 0, false
}
