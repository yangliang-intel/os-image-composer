# OS Image Composer Snapshot Feature

## Overview

The Snapshot feature enables **reproducible builds** by capturing exact package versions and dependencies from a successful build, then using that snapshot to build identical images later.

## Problem Statement

Without snapshots, rebuilding an image with the same template may produce different results if:
- Repository metadata has changed
- Package versions have been updated
- Dependencies have been resolved differently

The snapshot solves this by freezing exact versions at build time.

## Snapshot File Format

A snapshot is a JSON file containing:

```json
{
  "version": "1.0",
  "buildDate": "2026-03-28T17:10:42Z",
  "buildSystem": "os-image-composer/1.0",
  "template": {
    "name": "emt3-x86_64-minimal",
    "version": "1.0.0",
    "os": "edge-microvisor-toolkit",
    "dist": "emt3",
    "arch": "x86_64",
    "imageType": "iso"
  },
  "packageRepositories": [
    {
      "id": "primary",
      "codename": "emt3.0-base",
      "url": "https://files-rs.edgeorchestration.intel.com/files-edge-orch/microvisor/rpms/3.0/base",
      "type": "rpm"
    }
  ],
  "packages": {
    "essential": [
      {
        "name": "filesystem",
        "version": "1.1-21",
        "release": "emt3",
        "arch": "x86_64",
        "fullName": "filesystem-1.1-21.emt3.x86_64.rpm",
        "downloadUrl": "...",
        "sha256": "..."
      }
    ],
    "kernel": [
      {
        "name": "kernel",
        "version": "6.12.67",
        "release": "1.emt3",
        "arch": "x86_64",
        "fullName": "kernel-6.12.67-1.emt3.x86_64.rpm",
        "downloadUrl": "...",
        "sha256": "..."
      }
    ],
    "bootloader": [...],
    "system": [...]
  },
  "checksums": {
    "packageMetadata": "sha256...",
    "templateConfig": "sha256..."
  }
}
```

## Usage Scenarios

### Scenario 1: Create a Snapshot

```bash
# Build an image and save the exact package versions used
sudo -E ./os-image-composer build template.yml --snapshot-save my-build.snapshot.json
```

### Scenario 2: Reproduce from Snapshot

```bash
# Build an identical image using the saved snapshot
sudo -E ./os-image-composer build template.yml --snapshot-load my-build.snapshot.json
```

## Implementation Details

### 1. Snapshot Data Structure

**File**: `internal/config/snapshot/snapshot.go`

```go
type PackageSnapshot struct {
    Name         string `json:"name"`
    Version      string `json:"version"`
    Release      string `json:"release"`
    Arch         string `json:"arch"`
    FullName     string `json:"fullName"`
    DownloadURL  string `json:"downloadUrl"`
    SHA256       string `json:"sha256"`
}

type Snapshot struct {
    Version               string                      `json:"version"`
    BuildDate             string                      `json:"buildDate"`
    BuildSystem           string                      `json:"buildSystem"`
    Template              TemplateSnapshot            `json:"template"`
    PackageRepositories   []RepositorySnapshot        `json:"packageRepositories"`
    Packages              map[string][]PackageSnapshot `json:"packages"`
    Checksums             ChecksumsSnapshot           `json:"checksums"`
}
```

### 2. Save Snapshot

**When**: After image build package installation and SBOM matching

**Important**: Snapshot package contents are captured from the same installed-matched
package set used for SPDX generation (not the raw unresolved/download candidate list).
This avoids false diffs when multiple versions are temporarily resolved but only one
version is actually installed.

For ISO builds (where packages are copied into the ISO cache repository instead of
installed into a rootfs), snapshot package contents are synchronized to the final
`FullPkgList` package-file set after download/sorting. This keeps ISO snapshots
deterministic and aligned with the exact package files embedded in the ISO.

**Location**: In `PostProcess` or after `PreProcess`

```go
// In provider's PostProcess
if snapshotSavePath != "" {
    if err := snapshot.SaveSnapshot(template, snapshotSavePath); err != nil {
        log.Warnf("failed to save snapshot: %v", err)
    }
}
```

### 3. Load Snapshot

**When**: After template merge, before package download

**Location**: In `PreProcess` or beginning of `BuildImage`

```go
// In executeBuild after template loading
if snapshotLoadPath != "" {
    snap, err := snapshot.LoadSnapshot(snapshotLoadPath)
    if err != nil {
        return fmt.Errorf("failed to load snapshot: %w", err)
    }
    
    // Apply snapshot constraints to template
    if err := template.ApplySnapshot(snap); err != nil {
        return fmt.Errorf("failed to apply snapshot: %w", err)
    }
}
```

### 4. Apply Snapshot to Template

**Logic**:
1. Convert snapshot package list to version pinning constraints
2. Add constraints to `template.PackageRepositories[i].AllowPackages` with version specs
3. OR modify package resolution to skip repository queries and use snapshot directly

### 5. Determinism Checks

**Validation before applying snapshot**:
- Template matches snapshot template (same OS, dist, arch, imageType)
- Required repositories are available
- Repository URLs haven't changed (warning if changed)

## Integration Points

### Command-Line Flags

```go
var (
    snapshotSave string = ""  // Path to save snapshot after build
    snapshotLoad string = ""  // Path to load snapshot before build
)

buildCmd.Flags().StringVar(&snapshotSave, "snapshot-save", "",
    "Save the resolved package snapshot to this file after build")
buildCmd.Flags().StringVar(&snapshotLoad, "snapshot-load", "",
    "Load and use exact package versions from this snapshot file")
```

### Build Command Flow

```
1. Load template and merge with defaults
2. Load snapshot (if --snapshot-load provided)
3. Apply snapshot constraints to template
4. Initialize provider
5. PreProcess (download packages)
   → If snapshot loaded, validate against snapshot
   → If not, resolve packages normally
6. BuildImage
7. PostProcess
8. Save snapshot (if --snapshot-save provided)
```

## Benefits

✅ **Reproducible Builds**: Exact same packages every time  
✅ **Audit Trail**: Records what went into each image  
✅ **Offline Builds**: Can pre-cache packages from snapshot  
✅ **Version Pinning**: No surprise updates  
✅ **Compliance**: Document exact software versions for CVE tracking  

## Limitations & Considerations

⚠️ **Repository Changes**: If repository URL changes, snapshot cannot be applied  
⚠️ **Package Deletion**: If packages are deleted from repo, snapshot cannot be used  
⚠️ **Breaking Changes**: Template changes (new packages) require snapshot regeneration  
⚠️ **Expiration**: Snapshots may become incompatible with newer tool versions  

## Version Strategy

- Snapshots include tool version for forward compatibility checking
- Tool can warn if snapshot was created with different version
- Tools should be backward compatible with older snapshot formats

## Testing Strategy

1. **Unit Tests**: Snapshot marshal/unmarshal
2. **Integration Tests**: Build → Save → Load → Build comparison
3. **Reproducibility Tests**: Byte-for-byte image comparison (hash)
4. **Error Tests**: Missing repos, incompatible templates, corrupted snapshots
