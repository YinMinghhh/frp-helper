package model

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	AppName            = "frp-helper"
	DefaultFRPCVersion = "v0.68.0"
	DefaultBindAddr    = "127.0.0.1"
	RedactedValue      = "***REDACTED***"
)

type Manifest struct {
	ServerAddr string    `json:"serverAddr"`
	ServerPort int       `json:"serverPort"`
	AuthToken  string    `json:"authToken"`
	User       string    `json:"user,omitempty"`
	Services   []Service `json:"services"`
}

type Service struct {
	Name         string `json:"name"`
	ServerName   string `json:"serverName"`
	SecretKey    string `json:"secretKey"`
	BindPort     int    `json:"bindPort"`
	ServerUser   string `json:"serverUser,omitempty"`
	ProtocolHint string `json:"protocolHint,omitempty"`
	Disabled     bool   `json:"disabled,omitempty"`
	AccessUser   string `json:"accessUser,omitempty"`
}

type RuntimeState struct {
	SelectedVersion string                    `json:"selectedVersion,omitempty"`
	FRPCPath        string                    `json:"frpcPath,omitempty"`
	ConfigPath      string                    `json:"configPath,omitempty"`
	ManifestPath    string                    `json:"manifestPath,omitempty"`
	PID             int                       `json:"pid,omitempty"`
	Running         bool                      `json:"running"`
	LastStartTime   *time.Time                `json:"lastStartTime,omitempty"`
	LastStopTime    *time.Time                `json:"lastStopTime,omitempty"`
	LastExitTime    *time.Time                `json:"lastExitTime,omitempty"`
	LastError       string                    `json:"lastError,omitempty"`
	Services        map[string]ServiceRuntime `json:"services,omitempty"`
}

type ServiceRuntime struct {
	ServiceKey string     `json:"serviceKey"`
	Name       string     `json:"name"`
	BindAddr   string     `json:"bindAddr"`
	BindPort   int        `json:"bindPort"`
	Protocol   string     `json:"protocol"`
	Status     string     `json:"status"`
	Endpoint   string     `json:"endpoint"`
	Message    string     `json:"message,omitempty"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
}

type EndpointRow struct {
	ServiceKey string
	Name       string
	BindAddr   string
	BindPort   int
	Protocol   string
	Status     string
	Command    string
}

type ValidationErrors struct {
	Messages []string
}

func (e *ValidationErrors) Add(format string, args ...any) {
	e.Messages = append(e.Messages, fmt.Sprintf(format, args...))
}

func (e *ValidationErrors) Error() string {
	return strings.Join(e.Messages, "; ")
}

func (e *ValidationErrors) Empty() bool {
	return len(e.Messages) == 0
}

func ServiceKey(resolvedServerUser, serverName string) string {
	return fmt.Sprintf("%s:%s", resolvedServerUser, serverName)
}

func (s Service) ResolvedServerUser(globalUser string) string {
	if strings.TrimSpace(s.ServerUser) != "" {
		return strings.TrimSpace(s.ServerUser)
	}
	return strings.TrimSpace(globalUser)
}

func (s Service) Key(globalUser string) string {
	return ServiceKey(s.ResolvedServerUser(globalUser), strings.TrimSpace(s.ServerName))
}

func (s Service) BindAddress() string {
	return DefaultBindAddr
}

func (s Service) NormalizedProtocol() string {
	return strings.ToLower(strings.TrimSpace(s.ProtocolHint))
}

func (s Service) AccessCommand() string {
	addr := fmt.Sprintf("%s:%d", s.BindAddress(), s.BindPort)
	switch s.NormalizedProtocol() {
	case "ssh":
		user := strings.TrimSpace(s.AccessUser)
		if user == "" {
			user = "<username>"
		}
		return fmt.Sprintf("ssh -p %d %s@%s", s.BindPort, user, s.BindAddress())
	case "http":
		return fmt.Sprintf("http://%s", addr)
	case "https":
		return fmt.Sprintf("https://%s", addr)
	case "rdp":
		return addr
	default:
		return addr
	}
}

func (m Manifest) EnabledServices() []Service {
	services := make([]Service, 0, len(m.Services))
	for _, svc := range m.Services {
		if !svc.Disabled {
			services = append(services, svc)
		}
	}
	return services
}

func (m Manifest) SortedServices() []Service {
	services := append([]Service(nil), m.Services...)
	slices.SortFunc(services, func(a, b Service) int {
		return strings.Compare(a.Key(m.User), b.Key(m.User))
	})
	return services
}

func (m Manifest) Secrets() []string {
	secrets := []string{strings.TrimSpace(m.AuthToken)}
	for _, svc := range m.Services {
		secrets = append(secrets, strings.TrimSpace(svc.SecretKey))
	}
	return secrets
}
