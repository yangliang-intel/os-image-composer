// Package snapshot provides functionality to capture and restore exact package versions
// for reproducible image builds.
package snapshot

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/security"
)

var log = logger.Logger()

const SnapshotVersion = "1.0"

// PackageSnapshot captures the exact details of a package at build time
type PackageSnapshot struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Arch        string `json:"arch"`
	FullName    string `json:"fullName"`
	DownloadURL string `json:"downloadUrl,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	Source      string `json:"source,omitempty"` // Package source category: "essential", "kernel", "bootloader", "system"
}

// RepositorySnapshot captures repository configuration used during build
type RepositorySnapshot struct {
	ID       string `json:"id,omitempty"`
	Codename string `json:"codename"`
	URL      string `json:"url"`
	Type     string `json:"type"` // "rpm" or "deb"
}

// TemplateSnapshot captures template metadata
type TemplateSnapshot struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	OS        string `json:"os"`
	Dist      string `json:"dist"`
	Arch      string `json:"arch"`
	ImageType string `json:"imageType"`
}

// ChecksumsSnapshot provides integrity verification
type ChecksumsSnapshot struct {
	PackageMetadataHash string `json:"packageMetadataHash,omitempty"`
	TemplateConfigHash  string `json:"templateConfigHash,omitempty"`
}

// Snapshot is the root structure for a build snapshot
type Snapshot struct {
	Version               string                   `json:"version"`
	BuildDate             string                   `json:"buildDate"`
	BuildSystem           string                   `json:"buildSystem"`
	Template              TemplateSnapshot         `json:"template"`
	PackageRepositories   []RepositorySnapshot     `json:"packageRepositories"`
	Packages              map[string][]PackageSnapshot `json:"packages"` // Keys: "essential", "kernel", "bootloader", "system"
	Checksums             ChecksumsSnapshot        `json:"checksums"`
}

// NewSnapshot creates a snapshot from the current build template and resolved packages
func NewSnapshot(template *config.ImageTemplate) *Snapshot {
	buildDate := time.Now().UTC().Format(time.RFC3339)

	// Capture repository information
	repoSnapshots := make([]RepositorySnapshot, 0, len(template.PackageRepositories))
	for _, repo := range template.PackageRepositories {
		repoSnapshots = append(repoSnapshots, RepositorySnapshot{
			ID:       repo.ID,
			Codename: repo.Codename,
			URL:      repo.URL,
			Type:     "rpm", // Will be set properly during implementation
		})
	}

	// Organize packages by source
	packagesMap := make(map[string][]PackageSnapshot)
	packagesMap["all"] = convertPackageInfoToSnapshot(template.FullPkgListBom)

	return &Snapshot{
		Version:     SnapshotVersion,
		BuildDate:   buildDate,
		BuildSystem: fmt.Sprintf("os-image-composer/%s", "1.0"), // Will be replaced with actual version
		Template: TemplateSnapshot{
			Name:      template.Image.Name,
			Version:   template.Image.Version,
			OS:        template.Target.OS,
			Dist:      template.Target.Dist,
			Arch:      template.Target.Arch,
			ImageType: template.Target.ImageType,
		},
		PackageRepositories: repoSnapshots,
		Packages:            packagesMap,
		Checksums:          ChecksumsSnapshot{},
	}
}

// convertPackageInfoToSnapshot converts PackageInfo structs to PackageSnapshot
func convertPackageInfoToSnapshot(packages []ospackage.PackageInfo) []PackageSnapshot {
	snapshots := make([]PackageSnapshot, 0, len(packages))
	for _, pkg := range packages {
		snapshotPkg := PackageSnapshot{
			Name:        pkg.PkgName,
			Version:     pkg.Version,
			Arch:        pkg.Arch,
			FullName:    pkg.Name,
			DownloadURL: pkg.URL,
		}
		for _, cs := range pkg.Checksums {
			if cs.Algorithm == "sha256" || cs.Algorithm == "SHA256" {
				snapshotPkg.SHA256 = cs.Value
				break
			}
		}
		snapshots = append(snapshots, snapshotPkg)
	}
	return snapshots
}

// SaveSnapshot writes a snapshot to a JSON file
func SaveSnapshot(snapshot *Snapshot, filePath string) error {
	log.Infof("Saving snapshot to: %s", filePath)

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Write file securely, rejecting symlinks
	if err := security.SafeWriteFile(filePath, data, 0644, security.RejectSymlinks); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	log.Infof("Snapshot saved successfully (%d packages, %d repositories)",
		len(snapshot.Packages["all"]), len(snapshot.PackageRepositories))

	return nil
}

// LoadSnapshot reads a snapshot from a JSON file
func LoadSnapshot(filePath string) (*Snapshot, error) {
	log.Infof("Loading snapshot from: %s", filePath)

	data, err := security.SafeReadFile(filePath, security.RejectSymlinks)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot: %w", err)
	}

	log.Infof("Snapshot loaded successfully (version=%s, built=%s, packages=%d)",
		snap.Version, snap.BuildDate, len(snap.Packages["all"]))

	return &snap, nil
}

// ValidateSnapshot checks if a snapshot is compatible with the current template
func ValidateSnapshot(snap *Snapshot, template *config.ImageTemplate) error {
	// Check version compatibility
	if snap.Version != SnapshotVersion {
		log.Warnf("snapshot version %s differs from current version %s (may be incompatible)",
			snap.Version, SnapshotVersion)
	}

	// Check template compatibility
	if snap.Template.OS != template.Target.OS {
		return fmt.Errorf("snapshot OS (%s) does not match template OS (%s)",
			snap.Template.OS, template.Target.OS)
	}

	if snap.Template.Dist != template.Target.Dist {
		return fmt.Errorf("snapshot dist (%s) does not match template dist (%s)",
			snap.Template.Dist, template.Target.Dist)
	}

	if snap.Template.Arch != template.Target.Arch {
		return fmt.Errorf("snapshot arch (%s) does not match template arch (%s)",
			snap.Template.Arch, template.Target.Arch)
	}

	if snap.Template.ImageType != template.Target.ImageType {
		return fmt.Errorf("snapshot imageType (%s) does not match template imageType (%s)",
			snap.Template.ImageType, template.Target.ImageType)
	}

	log.Infof("Snapshot validation passed: %s/%s %s %s",
		snap.Template.OS, snap.Template.Dist, snap.Template.Arch, snap.Template.ImageType)

	return nil
}

// ApplySnapshot pins package versions in the template based on the snapshot
func ApplySnapshot(snap *Snapshot, template *config.ImageTemplate) error {
	if snap == nil {
		return fmt.Errorf("snapshot is nil")
	}

	if template == nil {
		return fmt.Errorf("template is nil")
	}

	// Validate snapshot compatibility
	if err := ValidateSnapshot(snap, template); err != nil {
		return err
	}

	// Build a map of package name -> version for quick lookup
	pinnedVersions := make(map[string]string)
	for _, pkg := range snap.Packages["all"] {
		// Use the version string directly as stored in the snapshot
		pinnedVersions[pkg.Name] = pkg.Version
		log.Debugf("Pinning package %s to version %s", pkg.Name, pkg.Version)
	}

	// Update template to pin these versions
	// This will be used during package resolution to enforce exact versions
	template.SnapshotPackageVersions = pinnedVersions

	log.Infof("Applied snapshot constraints: pinned %d packages", len(pinnedVersions))

	return nil
}

// Export snapshot as YAML for human readability
func (s *Snapshot) ToYAML() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal snapshot: %w", err)
	}
	return string(data), nil
}

// Summary returns a human-readable summary of the snapshot
func (s *Snapshot) Summary() string {
	return fmt.Sprintf(
		"Snapshot: %s/%s v%s (%s)\n"+
			"Built: %s\n"+
			"Packages: %d\n"+
			"Repositories: %d",
		s.Template.OS, s.Template.Dist, s.Template.Version, s.Template.ImageType,
		s.BuildDate,
		len(s.Packages["all"]),
		len(s.PackageRepositories),
	)
}
