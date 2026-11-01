//go:build windows

package util

func DiskFreeBytes(path string) uint64 {
	return 1024 * 1024 * 1024 * 10 // fallback
}
