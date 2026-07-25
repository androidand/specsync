package specsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ValidateChange represents a structural issue found in a change folder.
type ValidateIssue struct {
	Slug  string `json:"slug"`
	Field string `json:"field"`
	Error string `json:"error"`
}

// ValidateResult holds the validation report for all changes.
type ValidateResult struct {
	Issues []ValidateIssue `json:"issues,omitempty"`
	Total  int             `json:"total"`
}

// ValidateChanges scans all change folders for structural issues and returns
// a report. Checks: required files (proposal.md, tasks.md), valid metadata,
// well-formed stage mappings.
func ValidateChanges(openspecDir string) *ValidateResult {
	changesDir := filepath.Join(openspecDir, "changes")
	result := &ValidateResult{}

	validateDir(changesDir, false, result)
	validateDir(filepath.Join(changesDir, "archive"), true, result)

	result.Total = len(result.Issues)
	return result
}

func validateDir(dir string, archived bool, result *ValidateResult) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			result.Issues = append(result.Issues, ValidateIssue{
				Field: "dir",
				Error: fmt.Sprintf("read %s: %v", dir, err),
			})
		}
		return
	}

	for _, e := range entries {
		if !e.IsDir() || e.Name() == "archive" {
			continue
		}
		validateChange(filepath.Join(dir, e.Name()), e.Name(), archived, result)
	}
}

func validateChange(dir, slug string, archived bool, result *ValidateResult) {
	// Check proposal.md exists.
	proposalPath := filepath.Join(dir, "proposal.md")
	_, err := os.Stat(proposalPath)
	if os.IsNotExist(err) {
		result.Issues = append(result.Issues, ValidateIssue{
			Slug:  slug,
			Field: "proposal.md",
			Error: "missing (change folder requires proposal.md)",
		})
		return // can't validate further without proposal.md
	}

	// Check tasks.md exists.
	tasksPath := filepath.Join(dir, "tasks.md")
	_, err = os.Stat(tasksPath)
	if os.IsNotExist(err) {
		result.Issues = append(result.Issues, ValidateIssue{
			Slug:  slug,
			Field: "tasks.md",
			Error: "missing (change folder requires tasks.md)",
		})
	}

	// Validate .specsync/metadata.json if present.
	metaPath := filepath.Join(dir, ".specsync", "metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil && !os.IsNotExist(err) {
		result.Issues = append(result.Issues, ValidateIssue{
			Slug:  slug,
			Field: ".specsync/metadata.json",
			Error: fmt.Sprintf("read error: %v", err),
		})
	} else if err == nil {
		// Parse and validate JSON.
		var meta ChangeMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			result.Issues = append(result.Issues, ValidateIssue{
				Slug:  slug,
				Field: ".specsync/metadata.json",
				Error: fmt.Sprintf("invalid JSON: %v", err),
			})
		} else {
			// Validate stage if set.
			if meta.Stage != nil {
				if err := ValidateStage(*meta.Stage); err != nil {
					result.Issues = append(result.Issues, ValidateIssue{
						Slug:  slug,
						Field: ".specsync/metadata.json.stage",
						Error: err.Error(),
					})
				}
			}
		}
	}

}
