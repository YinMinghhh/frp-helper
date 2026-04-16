package frpc

import (
	"fmt"
	"runtime"
	"strings"
)

type Target struct {
	GOOS       string
	GOARCH     string
	ArchiveExt string
}

func ResolveTarget(goos, goarch string) (Target, error) {
	switch {
	case goos == "windows" && goarch == "amd64":
		return Target{GOOS: goos, GOARCH: goarch, ArchiveExt: ".zip"}, nil
	case goos == "darwin" && (goarch == "arm64" || goarch == "amd64"):
		return Target{GOOS: goos, GOARCH: goarch, ArchiveExt: ".tar.gz"}, nil
	case goos == "linux" && (goarch == "amd64" || goarch == "arm64"):
		return Target{GOOS: goos, GOARCH: goarch, ArchiveExt: ".tar.gz"}, nil
	default:
		return Target{}, fmt.Errorf("unsupported platform %s/%s", goos, goarch)
	}
}

func CurrentTarget() (Target, error) {
	return ResolveTarget(runtime.GOOS, runtime.GOARCH)
}

func (t Target) AssetName(version string) string {
	trimmed := strings.TrimPrefix(version, "v")
	return fmt.Sprintf("frp_%s_%s_%s%s", trimmed, t.GOOS, t.GOARCH, t.ArchiveExt)
}
