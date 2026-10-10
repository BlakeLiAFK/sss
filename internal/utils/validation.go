package utils

import (
	"net"
	"regexp"
	"strings"
)

var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func ValidBucketName(name string) bool {
	if !bucketPattern.MatchString(name) || strings.Contains(name, "..") || strings.Contains(name, ".-") || strings.Contains(name, "-.") || net.ParseIP(name) != nil {
		return false
	}
	switch name {
	case "api", "admin", "assets":
		return false
	}
	for _, suffix := range []string{".svg", ".ico", ".png", ".txt", ".webmanifest"} {
		if strings.HasSuffix(name, suffix) {
			return false
		}
	}
	return true
}

// Object keys are identifiers. Only actual traversal components are rejected.
func UnsafeObjectKey(key string) bool {
	if key == "" || strings.HasPrefix(key, "/") || strings.HasPrefix(key, "\\") || strings.ContainsRune(key, 0) {
		return true
	}
	for _, part := range strings.Split(strings.ReplaceAll(key, "\\", "/"), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}
