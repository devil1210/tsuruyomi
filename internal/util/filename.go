package util

import (
	"strings"
	"unicode/utf8"
)

// SanitizeFilename mutates the given filename to make it valid for a FAT filesystem,
// replacing any invalid characters with "_", matching Tachidesk's SafePath.buildValidFilename.
func SanitizeFilename(origName string) string {
	name := strings.Trim(origName, ". ")
	if name == "" {
		return "(invalid)"
	}

	var sb strings.Builder
	sb.Grow(len(name))

	for _, r := range name {
		if isValidFatFilenameChar(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}

	return truncateFilename(sb.String())
}

// isValidFatFilenameChar returns true if the rune is a valid FAT filename character.
func isValidFatFilenameChar(r rune) bool {
	if r <= 0x1f || r == 0x7f {
		return false
	}
	switch r {
	case '"', '*', '/', ':', '<', '>', '?', '\\', '|':
		return false
	default:
		return true
	}
}

// truncateFilename limits the string to 240 runes and 240 UTF-8 bytes constraint.
func truncateFilename(filename string) string {
	const maxLen = 240
	if len(filename) <= maxLen {
		return filename
	}

	var sb strings.Builder
	var bytesUsed int

	for _, r := range filename {
		runeBytes := utf8.RuneLen(r)
		if runeBytes == -1 {
			continue // Invalid rune
		}
		if bytesUsed+runeBytes > maxLen || sb.Len() >= maxLen {
			break
		}
		sb.WriteRune(r)
		bytesUsed += runeBytes
	}

	return sb.String()
}
