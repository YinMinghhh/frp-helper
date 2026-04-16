package frpc

import "strings"

func MapRuntimeError(raw string) string {
	msg, _ := InterpretRuntimeError(raw)
	return msg
}

func InterpretRuntimeError(raw string) (string, bool) {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return "", false
	}

	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "address already in use"),
		strings.Contains(lower, "port already used"),
		strings.Contains(lower, "bind: only one usage"),
		strings.Contains(lower, "bind failed"):
		return "local bindPort is already in use", true
	case strings.Contains(lower, "token") && (strings.Contains(lower, "auth") || strings.Contains(lower, "invalid") || strings.Contains(lower, "unauthorized")):
		return "frps authentication failed; check auth.token", true
	case strings.Contains(lower, "secret") && (strings.Contains(lower, "mismatch") || strings.Contains(lower, "invalid") || strings.Contains(lower, "auth")):
		return "stcp authentication failed; check secretKey", true
	case strings.Contains(lower, "doesn't exist"),
		strings.Contains(lower, "not exist"),
		strings.Contains(lower, "server name not found"),
		strings.Contains(lower, "no such proxy"):
		return "target stcp service does not exist; check serverName", true
	case strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no such host"),
		strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "network is unreachable"),
		strings.Contains(lower, "dial tcp"):
		return "failed to reach frps; check serverAddr/serverPort and network", true
	case strings.Contains(lower, "syntax error"),
		strings.Contains(lower, "invalid config"),
		strings.Contains(lower, "parse error"),
		strings.Contains(lower, "unmarshal error"):
		return "frpc configuration is invalid; check generated TOML", true
	default:
		return msg, false
	}
}
