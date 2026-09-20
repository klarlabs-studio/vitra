package packaging

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BuildAppDir writes an AppImage-ready AppDir under outDir (AppRun + usr/bin + .desktop).
// Producing a final .AppImage still requires an external tool (appimagetool); this
// layout is the portable intermediate those tools consume.
func BuildAppDir(spec Spec, binaryPath, outDir string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	if !hasTarget(spec, TargetLinuxAppImage) && !hasTarget(spec, TargetLinuxDir) {
		return Artifact{}, fmt.Errorf("BuildAppDir requires target %s or %s", TargetLinuxAppImage, TargetLinuxDir)
	}
	if binaryPath == "" {
		return Artifact{}, fmt.Errorf("binary path is required")
	}
	raw, err := os.ReadFile(binaryPath)
	if err != nil {
		return Artifact{}, err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])

	binName := sanitizeFileName(spec.Name)
	usrBin := filepath.Join(outDir, "usr", "bin")
	if err := os.MkdirAll(usrBin, 0o755); err != nil {
		return Artifact{}, err
	}
	binPath := filepath.Join(usrBin, binName)
	if err := os.WriteFile(binPath, raw, 0o755); err != nil {
		return Artifact{}, err
	}
	appRun := filepath.Join(outDir, "AppRun")
	if err := os.WriteFile(appRun, []byte("#!/bin/sh\nexec \"$(dirname \"$0\")/usr/bin/"+binName+"\" \"$@\"\n"), 0o755); err != nil {
		return Artifact{}, err
	}
	desktop := filepath.Join(outDir, binName+".desktop")
	body := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=AppRun
Icon=%s
Categories=Utility;
Terminal=false
`, spec.Name, binName)
	if err := os.WriteFile(desktop, []byte(body), 0o644); err != nil {
		return Artifact{}, err
	}

	return Artifact{
		Target: TargetLinuxAppImage,
		Path:   outDir,
		SHA256: digest,
		Signed: false,
	}, nil
}

// BuildDeb writes a Debian binary package (.deb) using only the Go standard library.
func BuildDeb(spec Spec, binaryPath, outPath string) (Artifact, error) {
	if err := spec.Validate(); err != nil {
		return Artifact{}, err
	}
	if !hasTarget(spec, TargetLinuxDeb) {
		return Artifact{}, fmt.Errorf("BuildDeb requires target %s", TargetLinuxDeb)
	}
	if binaryPath == "" {
		return Artifact{}, fmt.Errorf("binary path is required")
	}
	raw, err := os.ReadFile(binaryPath)
	if err != nil {
		return Artifact{}, err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])

	binName := sanitizeFileName(spec.Name)
	pkgName := debianName(spec.AppID, binName)
	arch := spec.Arch
	if arch == "" {
		arch = DefaultArch()
	}

	installedSize := (len(raw) + 1023) / 1024
	control := fmt.Sprintf(`Package: %s
Version: %s
Section: utils
Priority: optional
Architecture: %s
Maintainer: Vitra Packaging <vitra@klarlabs.de>
Installed-Size: %d
Description: %s
 Secure Go + web desktop application packaged by Vitra.
`, pkgName, spec.Version, arch, installedSize, spec.Name)

	controlTGZ, err := tarGz(map[string]fileEntry{
		"control": {data: []byte(control), mode: 0o644},
	})
	if err != nil {
		return Artifact{}, err
	}

	desktopPath := "usr/share/applications/" + sanitizeFileName(spec.AppID) + ".desktop"
	desktopBody := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=/usr/bin/%s
Categories=Utility;
Terminal=false
`, spec.Name, binName)
	dataTGZ, err := tarGz(map[string]fileEntry{
		"usr/bin/" + binName: {data: raw, mode: 0o755},
		desktopPath:          {data: []byte(desktopBody), mode: 0o644},
	})
	if err != nil {
		return Artifact{}, err
	}

	if outPath == "" {
		return Artifact{}, fmt.Errorf("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return Artifact{}, err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = f.Close() }()
	if err := writeAr(f, []arMember{
		{name: "debian-binary", data: []byte("2.0\n")},
		{name: "control.tar.gz", data: controlTGZ},
		{name: "data.tar.gz", data: dataTGZ},
	}); err != nil {
		return Artifact{}, err
	}

	return Artifact{
		Target: TargetLinuxDeb,
		Path:   outPath,
		SHA256: digest,
		Signed: false,
	}, nil
}

func hasTarget(spec Spec, want Target) bool {
	for _, t := range spec.Targets {
		if t == want {
			return true
		}
	}
	return false
}

func debianName(appID, fallback string) string {
	s := strings.ToLower(appID)
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '+':
			return r
		default:
			return '-'
		}
	}, s)
	s = strings.Trim(s, "-")
	if s == "" {
		s = strings.ToLower(fallback)
	}
	if s == "" {
		return "vitra-app"
	}
	return s
}

type fileEntry struct {
	data []byte
	mode int64
}

func tarGz(files map[string]fileEntry) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	// Stable order for digests/tests.
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	now := time.Unix(0, 0).UTC()
	for _, name := range names {
		fe := files[name]
		hdr := &tar.Header{
			Name:    name,
			Mode:    fe.mode,
			Size:    int64(len(fe.data)),
			ModTime: now,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(fe.data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type arMember struct {
	name string
	data []byte
}

func writeAr(w io.Writer, members []arMember) error {
	if _, err := io.WriteString(w, "!<arch>\n"); err != nil {
		return err
	}
	for _, m := range members {
		name := m.name
		if len(name) > 16 {
			name = name[:16]
		}
		hdr := fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d`\n",
			name, 0, 0, 0, 0o100644, len(m.data))
		if _, err := io.WriteString(w, hdr); err != nil {
			return err
		}
		if _, err := w.Write(m.data); err != nil {
			return err
		}
		if len(m.data)%2 == 1 {
			if _, err := w.Write([]byte{'\n'}); err != nil {
				return err
			}
		}
	}
	return nil
}
