package isomaker

import (
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage"
)

func TestSyncFullPkgListBomWithFullPkgList(t *testing.T) {
	tests := []struct {
		name            string
		fullPkgList     []string
		fullPkgListBom  []ospackage.PackageInfo
		expectedOrdered []string
		expectedLen     int
	}{
		{
			name:        "drops_extra_metadata_not_in_final_package_list",
			fullPkgList: []string{"pkg-a.rpm", "pkg-b.rpm"},
			fullPkgListBom: []ospackage.PackageInfo{
				{Name: "pkg-a.rpm", PkgName: "pkg-a", Version: "1", Arch: "x86_64"},
				{Name: "pkg-b.rpm", PkgName: "pkg-b", Version: "1", Arch: "x86_64"},
				{Name: "bootstrap-only.rpm", PkgName: "bootstrap-only", Version: "1", Arch: "x86_64"},
			},
			expectedOrdered: []string{"pkg-a.rpm", "pkg-b.rpm"},
			expectedLen:     2,
		},
		{
			name:        "reorders_bom_to_match_final_package_file_order",
			fullPkgList: []string{"pkg-b.rpm", "pkg-a.rpm"},
			fullPkgListBom: []ospackage.PackageInfo{
				{Name: "pkg-a.rpm", PkgName: "pkg-a", Version: "1", Arch: "x86_64"},
				{Name: "pkg-b.rpm", PkgName: "pkg-b", Version: "1", Arch: "x86_64"},
			},
			expectedOrdered: []string{"pkg-b.rpm", "pkg-a.rpm"},
			expectedLen:     2,
		},
		{
			name:        "keeps_available_entries_when_some_metadata_missing",
			fullPkgList: []string{"pkg-a.rpm", "pkg-missing.rpm"},
			fullPkgListBom: []ospackage.PackageInfo{
				{Name: "pkg-a.rpm", PkgName: "pkg-a", Version: "1", Arch: "x86_64"},
			},
			expectedOrdered: []string{"pkg-a.rpm"},
			expectedLen:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &config.ImageTemplate{
				FullPkgList:    tt.fullPkgList,
				FullPkgListBom: tt.fullPkgListBom,
			}

			syncFullPkgListBomWithFullPkgList(template)

			if len(template.FullPkgListBom) != tt.expectedLen {
				t.Fatalf("unexpected synchronized BOM length: got %d, want %d", len(template.FullPkgListBom), tt.expectedLen)
			}

			for index, expectedName := range tt.expectedOrdered {
				if template.FullPkgListBom[index].Name != expectedName {
					t.Fatalf("unexpected package order at index %d: got %s, want %s", index, template.FullPkgListBom[index].Name, expectedName)
				}
			}
		})
	}
}
