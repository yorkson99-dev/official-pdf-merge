package main

import (
    "path/filepath"
    "strings"
)

// needsMixedFileWarning is deliberately a *preflight user notification*
// decision, not an antivirus detection rule. Only the selected files in the
// final merge order count; excluded ZIP entries are never included.
func needsMixedFileWarning(names []string) bool {
    if len(names) <= 3 {
        return false
    }
    for _, name := range names {
        if !strings.EqualFold(filepath.Ext(name), ".pdf") {
            return true
        }
    }
    return false
}

func needsMixedZIPWarning(entries []officialEntry) bool {
    names := make([]string, len(entries))
    for i, entry := range entries {
        names[i] = entry.name
    }
    return needsMixedFileWarning(names)
}
