//go:build !windows

package util

import "syscall"

func DiskFreeBytes(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 1024 * 1024 * 1024 * 10 // fallback 10GB
	}
	return stat.Bavail * uint64(stat.Bsize)
}
