package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/sahilm/fuzzy"
)

type sortMode int

const (
	sortSize sortMode = iota
	sortAge
	sortEcosystem
	sortName
)

type riskFilter int

const (
	riskAll riskFilter = iota
	riskFilterLow
	riskFilterMedium
	riskFilterHigh
)

type filterState struct {
	fuzzyActive bool
	fuzzyText   string
	sort        sortMode
	risk        riskFilter
}

func (fs filterState) isActive() bool {
	return fs.fuzzyText != "" || fs.sort != sortSize || fs.risk != riskAll
}

func (fs filterState) label() string {
	var parts []string
	switch fs.sort {
	case sortSize:
		// default, don't show
	case sortAge:
		parts = append(parts, "sort:age")
	case sortEcosystem:
		parts = append(parts, "sort:ecosystem")
	case sortName:
		parts = append(parts, "sort:name")
	}
	switch fs.risk {
	case riskAll:
		// default, don't show
	case riskFilterLow:
		parts = append(parts, "risk:low")
	case riskFilterMedium:
		parts = append(parts, "risk:medium")
	case riskFilterHigh:
		parts = append(parts, "risk:high")
	}
	if fs.fuzzyText != "" {
		parts = append(parts, fmt.Sprintf("fuzzy:'%s'", fs.fuzzyText))
	}
	return strings.Join(parts, " ")
}

// targetSource implements sahilm/fuzzy.Source for path-based matching.
type targetSource []flatTarget

func (t targetSource) String(i int) string { return t[i].target.Path }
func (t targetSource) Len() int            { return len(t) }

// recomputeVisible applies risk filter, fuzzy filter, and sorting to produce visible indices.
func recomputeVisible(targets []flatTarget, fs filterState, thresholdBytes int64) []int {
	// Start with all indices
	indices := make([]int, 0, len(targets))
	for i := range targets {
		indices = append(indices, i)
	}

	// Apply risk filter
	if fs.risk != riskAll {
		var riskLevel domain.RiskLevel
		switch fs.risk {
		case riskFilterLow:
			riskLevel = domain.RiskLow
		case riskFilterMedium:
			riskLevel = domain.RiskMedium
		case riskFilterHigh:
			riskLevel = domain.RiskHigh
		}
		filtered := indices[:0]
		for _, idx := range indices {
			if targets[idx].target.Risk == riskLevel {
				filtered = append(filtered, idx)
			}
		}
		indices = filtered
	}

	// Apply fuzzy filter
	if fs.fuzzyText != "" {
		// Build a subset source for fuzzy matching
		subset := make([]flatTarget, len(indices))
		for i, idx := range indices {
			subset[i] = targets[idx]
		}
		matches := fuzzy.FindFrom(fs.fuzzyText, targetSource(subset))
		matched := make([]int, 0, len(matches))
		for _, m := range matches {
			matched = append(matched, indices[m.Index])
		}
		indices = matched
	}

	// Sort
	sort.Slice(indices, func(a, b int) bool {
		ta := targets[indices[a]]
		tb := targets[indices[b]]
		switch fs.sort {
		case sortSize:
			return ta.target.SizeBytes > tb.target.SizeBytes
		case sortAge:
			return ta.target.LastAccessed.Before(tb.target.LastAccessed)
		case sortEcosystem:
			return ta.ecosystem < tb.ecosystem
		case sortName:
			return ta.target.Path < tb.target.Path
		}
		return false
	})

	return indices
}
