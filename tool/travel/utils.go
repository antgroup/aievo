package travel

import (
	"strings"
)

// extractBeforeParenthesis extracts the text before parenthesis
func extractBeforeParenthesis(s string) string {
	if idx := strings.Index(s, "("); idx != -1 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

// getValidNameCity extracts name and city from a formatted string
func getValidNameCity(info string) (string, string) {
	// Pattern: name, city or name, city(state)
	parts := strings.Split(info, ",")
	if len(parts) >= 2 {
		name := strings.TrimSpace(parts[0])
		cityPart := strings.TrimSpace(parts[1])
		city := extractBeforeParenthesis(cityPart)
		return name, city
	}
	return "-", "-"
}
