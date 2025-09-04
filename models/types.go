package models

import (
	"unicode/utf8"
)

func ValidKey(k string) bool {
	if l := utf8.RuneCountInString(k); l < 1 || l > 64 {
		return false
	}
	for _, r := range k {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}
