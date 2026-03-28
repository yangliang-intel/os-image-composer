package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage"
)

func makeTemplate(osName, dist, arch, imageType string) *config.ImageTemplate {
	return &config.ImageTemplate{
		Image: config.ImageInfo{
			Name:    "test-image",
			Version: "1.0.0",
		},
		Target: config.TargetInfo{
			OS:        osName,
			Dist:      dist,
			Arch:      arch,
			ImageType: imageType,
		},
	}
}

func makePackages() []ospackage.PackageInfo {
	return []ospackage.PackageInfo{
		{
			Name:    "kernel-6.12.67-1.emt3.x86_64.rpm",
			PkgName: "kernel",
			Version: "6.12.67-1.emt3",
			Arch:    "x86_64",
			URL:     "https://example.com/kernel-6.12.67-1.emt3.x86_64.rpm",
			Checksums: []ospackage.Checksum{
				{Algorithm: "sha256", Value: "abc123"},
			},
		},
		{
			Name:    "systemd-255-31.emt3.x86_64.rpm",
			PkgName: "systemd",
			Version: "255-31.emt3",
			Arch:    "x86_64",
			URL:     "https://example.com/systemd-255-31.emt3.x86_64.rpm",
		},
	}
}

func TestNewSnapshot(t *testing.T) {
	tmpl := makeTemplate("edge-microvisor-toolkit", "emt3", "x86_64", "iso")
	tmpl.FullPkgListBom = makePackages()

	snap := NewSnapshot(tmpl)

	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.Version != SnapshotVersion {
		t.Errorf("version: got %s, want %s", snap.Version, SnapshotVersion)
	}
	if snap.Template.OS != "edge-microvisor-toolkit" {
		t.Errorf("OS: got %s, want edge-microvisor-toolkit", snap.Template.OS)
	}
	if snap.Template.Dist != "emt3" {
		t.Errorf("dist: got %s, want emt3", snap.Template.Dist)
	}
	if snap.Template.Arch != "x86_64" {
		t.Errorf("arch: got %s, want x86_64", snap.Template.Arch)
	}
	pkgs := snap.Packages["all"]
	if len(pkgs) != 2 {
		t.Fatalf("packages: got %d, want 2", len(pkgs))
	}
	if pkgs[0].Name != "kernel" {
		t.Errorf("first package name: got %s, want kernel", pkgs[0].Name)
	}
	if pkgs[0].Version != "6.12.67-1.emt3" {
		t.Errorf("first package version: got %s, want 6.12.67-1.emt3", pkgs[0].Version)
	}
	if pkgs[0].SHA256 != "abc123" {
		t.Errorf("first package sha256: got %s, want abc123", pkgs[0].SHA256)
	}
}

func TestSaveAndLoadSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.snapshot.json")

	tmpl := makeTemplate("edge-microvisor-toolkit", "emt3", "x86_64", "raw")
	tmpl.FullPkgListBom = makePackages()

	snap := NewSnapshot(tmpl)

	if err := SaveSnapshot(snap, path); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("snapshot file not created: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading snapshot file: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("snapshot is not valid JSON: %v", err)
	}

	loaded, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot failed: %v", err)
	}
	if loaded.Template.OS != snap.Template.OS {
		t.Errorf("OS mismatch: got %s, want %s", loaded.Template.OS, snap.Template.OS)
	}
	if len(loaded.Packages["all"]) != len(snap.Packages["all"]) {
		t.Errorf("package count mismatch: got %d, want %d",
			len(loaded.Packages["all"]), len(snap.Packages["all"]))
	}
}

func TestValidateSnapshot(t *testing.T) {
	tests := []struct {
		name      string
		snapOS    string
		snapDist  string
		snapArch  string
		snapType  string
		tmplOS    string
		tmplDist  string
		tmplArch  string
		tmplType  string
		expectErr bool
	}{
		{
			name:     "matching template",
			snapOS:   "edge-microvisor-toolkit", snapDist: "emt3", snapArch: "x86_64", snapType: "iso",
			tmplOS:   "edge-microvisor-toolkit", tmplDist: "emt3", tmplArch: "x86_64", tmplType: "iso",
		},
		{
			name:      "OS mismatch",
			snapOS:    "azure-linux", snapDist: "azl3", snapArch: "x86_64", snapType: "raw",
			tmplOS:    "edge-microvisor-toolkit", tmplDist: "emt3", tmplArch: "x86_64", tmplType: "raw",
			expectErr: true,
		},
		{
			name:      "dist mismatch",
			snapOS:    "edge-microvisor-toolkit", snapDist: "emt3", snapArch: "x86_64", snapType: "raw",
			tmplOS:    "edge-microvisor-toolkit", tmplDist: "emt4", tmplArch: "x86_64", tmplType: "raw",
			expectErr: true,
		},
		{
			name:      "arch mismatch",
			snapOS:    "edge-microvisor-toolkit", snapDist: "emt3", snapArch: "x86_64", snapType: "raw",
			tmplOS:    "edge-microvisor-toolkit", tmplDist: "emt3", tmplArch: "aarch64", tmplType: "raw",
			expectErr: true,
		},
		{
			name:      "imageType mismatch",
			snapOS:    "edge-microvisor-toolkit", snapDist: "emt3", snapArch: "x86_64", snapType: "raw",
			tmplOS:    "edge-microvisor-toolkit", tmplDist: "emt3", tmplArch: "x86_64", tmplType: "iso",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := &Snapshot{
				Version: SnapshotVersion,
				Template: TemplateSnapshot{
					OS:        tt.snapOS,
					Dist:      tt.snapDist,
					Arch:      tt.snapArch,
					ImageType: tt.snapType,
				},
			}
			tmpl := makeTemplate(tt.tmplOS, tt.tmplDist, tt.tmplArch, tt.tmplType)

			err := ValidateSnapshot(snap, tmpl)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestApplySnapshot(t *testing.T) {
	tmpl := makeTemplate("edge-microvisor-toolkit", "emt3", "x86_64", "raw")

	snap := &Snapshot{
		Version: SnapshotVersion,
		Template: TemplateSnapshot{
			OS:        "edge-microvisor-toolkit",
			Dist:      "emt3",
			Arch:      "x86_64",
			ImageType: "raw",
		},
		Packages: map[string][]PackageSnapshot{
			"all": {
				{Name: "kernel", Version: "6.12.67-1.emt3"},
				{Name: "systemd", Version: "255-31.emt3"},
			},
		},
	}

	if err := ApplySnapshot(snap, tmpl); err != nil {
		t.Fatalf("ApplySnapshot failed: %v", err)
	}

	if tmpl.SnapshotPackageVersions == nil {
		t.Fatal("expected SnapshotPackageVersions to be set")
	}
	if tmpl.SnapshotPackageVersions["kernel"] != "6.12.67-1.emt3" {
		t.Errorf("kernel version: got %s, want 6.12.67-1.emt3",
			tmpl.SnapshotPackageVersions["kernel"])
	}
	if tmpl.SnapshotPackageVersions["systemd"] != "255-31.emt3" {
		t.Errorf("systemd version: got %s, want 255-31.emt3",
			tmpl.SnapshotPackageVersions["systemd"])
	}
}

func TestApplySnapshot_NilInputs(t *testing.T) {
	tests := []struct {
		name string
		snap *Snapshot
		tmpl *config.ImageTemplate
	}{
		{"nil snapshot", nil, makeTemplate("a", "b", "c", "d")},
		{"nil template", &Snapshot{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ApplySnapshot(tt.snap, tt.tmpl)
			if err == nil {
				t.Error("expected error for nil input, got nil")
			}
		})
	}
}

func TestLoadSnapshot_InvalidFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{"non-existent file", "/tmp/does-not-exist-snapshot.json", true},
		{"invalid json", "", true}, // Will be set to a file with invalid content
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.filePath
			if path == "" {
				// Create temp file with invalid JSON
				dir := t.TempDir()
				path = filepath.Join(dir, "bad.json")
				if err := os.WriteFile(path, []byte("not valid json"), 0644); err != nil {
					t.Fatalf("creating test file: %v", err)
				}
			}
			_, err := LoadSnapshot(path)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestSnapshotSummary(t *testing.T) {
	snap := &Snapshot{
		Version:   "1.0",
		BuildDate: "2026-03-28T00:00:00Z",
		Template: TemplateSnapshot{
			OS:        "edge-microvisor-toolkit",
			Dist:      "emt3",
			Version:   "1.0.0",
			ImageType: "iso",
		},
		Packages: map[string][]PackageSnapshot{
			"all": {{Name: "kernel", Version: "6.12"}},
		},
		PackageRepositories: []RepositorySnapshot{
			{URL: "https://example.com/repo"},
		},
	}

	summary := snap.Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestConvertPackageInfoToSnapshot_SHA256Priority(t *testing.T) {
	pkgs := []ospackage.PackageInfo{
		{
			PkgName: "test-pkg",
			Version: "1.0",
			Arch:    "x86_64",
			URL:     "https://example.com/test.rpm",
			Checksums: []ospackage.Checksum{
				{Algorithm: "md5", Value: "md5val"},
				{Algorithm: "sha256", Value: "sha256val"},
			},
		},
	}

	result := convertPackageInfoToSnapshot(pkgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 package, got %d", len(result))
	}
	if result[0].SHA256 != "sha256val" {
		t.Errorf("expected sha256val, got %s", result[0].SHA256)
	}
}
