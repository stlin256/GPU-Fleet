package version

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChangelogFromFileParsesLocalizedMarkdown(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "CHANGELOG.md")
	raw := `# Changelog

## [1.2.3] - 2026-06-06

### Title / 标题

- zh-CN: 中文标题
- en-US: English title

### Added / 新增

- zh-CN: 中文新增
- en-US: English added

### Changed / 变更

- zh-CN: 中文变更
- en-US: English changed

### Fixed / 修复

- zh-CN: 中文修复
- en-US: English fixed
`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}

	entries, err := ChangelogFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Version != "1.2.3" || entry.Date != "2026-06-06" || entry.Title != "中文标题" || entry.TitleEN != "English title" {
		t.Fatalf("unexpected entry metadata: %+v", entry)
	}
	if entry.Added[0] != "中文新增" || entry.AddedEN[0] != "English added" || entry.Changed[0] != "中文变更" || entry.ChangedEN[0] != "English changed" || entry.Fixed[0] != "中文修复" || entry.FixedEN[0] != "English fixed" {
		t.Fatalf("unexpected localized content: %+v", entry)
	}
}
