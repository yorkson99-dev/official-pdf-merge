package main

import (
    "path/filepath"
    "strings"
)

// countEditableDocuments counts documents which require Microsoft Office
// conversion (all supported non-PDF formats). Pure PDF attachments do not
// count toward the user warning threshold, regardless of total file count.
func countEditableDocuments(names []string) int {
    count := 0
    for _, name := range names {
        if !strings.EqualFold(filepath.Ext(name), ".pdf") {
            count++
        }
    }
    return count
}

// User warning threshold: strictly more than TWO Office conversions in a
// single operation. This is a user-configurable caution, not a guarantee
// that Apex One will allow any smaller task.
func needsMixedFileWarning(names []string) bool {
    return countEditableDocuments(names) > 2
}

func countEditableZIPDocuments(entries []officialEntry) int {
    count := 0
    for _, entry := range entries {
        if !strings.EqualFold(filepath.Ext(entry.name), ".pdf") {
            count++
        }
    }
    return count
}

// Only analyzed/selected ZIP entries are passed here; excluded opinion sheets,
// drafts and other ZIP items do not contribute to the threshold.
func needsMixedZIPWarning(entries []officialEntry) bool {
    return countEditableZIPDocuments(entries) > 2
}
