package frpc

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"frp-helper/internal/store"
)

const defaultReleaseBaseURL = "https://github.com/fatedier/frp/releases/download"

type Installer struct {
	store  *store.Store
	client *http.Client
}

func NewInstaller(s *store.Store) *Installer {
	return &Installer{
		store:  s,
		client: &http.Client{},
	}
}

func (i *Installer) Install(ctx context.Context, version, archivePath, baseURL string) (string, error) {
	version = normalizeVersion(version)
	target, err := CurrentTarget()
	if err != nil {
		return "", err
	}

	destPath := i.store.FRPCPath(version)
	if fileExists(destPath) {
		return destPath, nil
	}

	archiveFile, cleanup, err := i.loadArchive(ctx, version, target, archivePath, baseURL)
	if err != nil {
		return "", err
	}
	defer cleanup()

	tmpDir, err := os.MkdirTemp("", "frpc-install-*")
	if err != nil {
		return "", fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractArchive(archiveFile, tmpDir, target.ArchiveExt); err != nil {
		return "", err
	}

	sourcePath, err := findFRPCBinary(tmpDir)
	if err != nil {
		return "", err
	}

	if err := copyFileAtomic(sourcePath, destPath, 0o755); err != nil {
		return "", err
	}
	return destPath, nil
}

func (i *Installer) loadArchive(ctx context.Context, version string, target Target, archivePath, baseURL string) (string, func(), error) {
	if archivePath != "" {
		return archivePath, func() {}, nil
	}

	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = defaultReleaseBaseURL
	}

	asset := target.AssetName(version)
	url := buildDownloadURL(base, version, asset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("build download request: %w", err)
	}

	resp, err := i.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}

	tmp, err := os.CreateTemp("", "frpc-archive-*"+target.ArchiveExt)
	if err != nil {
		return "", nil, fmt.Errorf("create temp archive: %w", err)
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("save archive: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("close archive: %w", err)
	}

	return tmp.Name(), func() { _ = os.Remove(tmp.Name()) }, nil
}

func buildDownloadURL(baseURL, version, asset string) string {
	if strings.Contains(baseURL, "%s") {
		return fmt.Sprintf(baseURL, version, asset)
	}
	return strings.TrimRight(baseURL, "/") + "/" + version + "/" + asset
}

func extractArchive(path, destDir, ext string) error {
	switch ext {
	case ".zip":
		return extractZip(path, destDir)
	case ".tar.gz":
		return extractTarGz(path, destDir)
	default:
		return fmt.Errorf("unsupported archive format %s", ext)
	}
}

func extractZip(path, destDir string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	defer r.Close()

	for _, file := range r.File {
		targetPath := filepath.Join(destDir, file.Name)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
			return fmt.Errorf("create directory %s: %w", targetPath, err)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o700); err != nil {
				return fmt.Errorf("create directory %s: %w", targetPath, err)
			}
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}
		if err := writeReader(targetPath, rc, file.Mode()); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}
	return nil
}

func extractTarGz(path, destDir string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open tar.gz archive: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		targetPath := filepath.Join(destDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o700); err != nil {
				return fmt.Errorf("create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
				return fmt.Errorf("create directory for %s: %w", targetPath, err)
			}
			if err := writeReader(targetPath, tr, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}
}

func writeReader(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create file %s: %w", path, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return fmt.Errorf("write file %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close file %s: %w", path, err)
	}
	return nil
}

func findFRPCBinary(root string) (string, error) {
	want := "frpc"
	if isWindows() {
		want += ".exe"
	}

	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == want {
			found = path
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("scan extracted archive: %w", err)
	}
	if found == "" {
		return "", fmt.Errorf("frpc binary not found in extracted archive")
	}
	return found, nil
}

func copyFileAtomic(src, dst string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("create directory for %s: %w", dst, err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source binary %s: %w", src, err)
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp binary: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close binary: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "v0.68.0"
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func isWindows() bool {
	return filepath.Separator == '\\'
}
