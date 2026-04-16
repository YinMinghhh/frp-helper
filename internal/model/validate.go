package model

import (
	"fmt"
	"strings"
)

func (m Manifest) Validate() error {
	errs := &ValidationErrors{}

	if strings.TrimSpace(m.ServerAddr) == "" {
		errs.Add("serverAddr is required")
	}
	if m.ServerPort < 1 || m.ServerPort > 65535 {
		errs.Add("serverPort must be between 1 and 65535")
	}
	if strings.TrimSpace(m.AuthToken) == "" {
		errs.Add("authToken is required")
	}

	serviceKeys := map[string]struct{}{}
	bindPorts := map[int]struct{}{}
	for idx, svc := range m.Services {
		label := fmt.Sprintf("services[%d]", idx)
		if strings.TrimSpace(svc.Name) == "" {
			errs.Add("%s.name is required", label)
		}
		if strings.TrimSpace(svc.ServerName) == "" {
			errs.Add("%s.serverName is required", label)
		}
		if strings.TrimSpace(svc.SecretKey) == "" {
			errs.Add("%s.secretKey is required", label)
		}
		if svc.BindPort < 1 || svc.BindPort > 65535 {
			errs.Add("%s.bindPort must be between 1 and 65535", label)
		}

		key := svc.Key(m.User)
		if _, ok := serviceKeys[key]; ok {
			errs.Add("duplicate service-key detected: %s", key)
		}
		serviceKeys[key] = struct{}{}

		if _, ok := bindPorts[svc.BindPort]; ok {
			errs.Add("duplicate bindPort detected: %d", svc.BindPort)
		}
		bindPorts[svc.BindPort] = struct{}{}
	}

	if errs.Empty() {
		return nil
	}
	return errs
}
