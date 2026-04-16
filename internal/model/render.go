package model

import (
	"fmt"
	"strconv"
	"strings"
)

func RenderTOML(m Manifest) string {
	var b strings.Builder

	writeKV(&b, "serverAddr", m.ServerAddr)
	writeInt(&b, "serverPort", m.ServerPort)
	b.WriteByte('\n')

	if strings.TrimSpace(m.User) != "" {
		writeKV(&b, "user", m.User)
	}
	writeBool(&b, "loginFailExit", false)
	b.WriteByte('\n')

	writeKV(&b, "auth.method", "token")
	writeKV(&b, "auth.token", m.AuthToken)

	for _, svc := range m.SortedServices() {
		if svc.Disabled {
			continue
		}
		b.WriteString("\n[[visitors]]\n")
		writeKV(&b, "name", svc.Name)
		writeKV(&b, "type", "stcp")
		resolvedUser := svc.ResolvedServerUser(m.User)
		if resolvedUser != "" {
			writeKV(&b, "serverUser", resolvedUser)
		}
		writeKV(&b, "serverName", svc.ServerName)
		writeKV(&b, "secretKey", svc.SecretKey)
		writeKV(&b, "bindAddr", svc.BindAddress())
		writeInt(&b, "bindPort", svc.BindPort)
	}

	return b.String()
}

func writeKV(b *strings.Builder, key, value string) {
	fmt.Fprintf(b, "%s = %s\n", key, strconv.Quote(value))
}

func writeInt(b *strings.Builder, key string, value int) {
	fmt.Fprintf(b, "%s = %d\n", key, value)
}

func writeBool(b *strings.Builder, key string, value bool) {
	fmt.Fprintf(b, "%s = %t\n", key, value)
}
