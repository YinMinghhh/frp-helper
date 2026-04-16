package frpc

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func VerifyConfig(ctx context.Context, frpcPath, configPath string) (string, error) {
	cmd := exec.CommandContext(ctx, frpcPath, "verify", "-c", configPath)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("verify config: %w", err)
		}
		return text, fmt.Errorf("verify config: %w", err)
	}
	return text, nil
}
