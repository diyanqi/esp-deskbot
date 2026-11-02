package iotda

import (
	"crypto/subtle"
	"sort"
	"strings"
)

func sortStrings(values []string) {
	sort.Strings(values)
}

func joinStrings(values []string) string {
	return strings.Join(values, "")
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
