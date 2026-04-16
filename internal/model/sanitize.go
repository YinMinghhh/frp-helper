package model

import "strings"

func Sanitize(text string, secrets []string) string {
	sanitized := text
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, secret, RedactedValue)
	}
	return sanitized
}
