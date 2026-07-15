package config

import (
	"fmt"
	"time"
)

// SpecSync configuration for priority-driven dispatch and state management.
// Integrates specsync workflow metadata (.specsync/metadata.json) with Skein's dispatcher.
type SpecSyncConfig struct {
	// Enabled activates specsync priority/stage integration.
	// When false, specsync metadata is ignored (backward compatible).
	Enabled bool `yaml:"enabled"`

	// Board configuration for automatic GitHub Projects sync.
	Board *SpecSyncBoard `yaml:"board,omitempty"`

	// Dispatcher configuration for priority-based work selection.
	Dispatcher *SpecSyncDispatcher `yaml:"dispatcher,omitempty"`

	// Priority model: how to interpret priorities.
	// "specsync" (1-100), "p1-p5" (legacy), "hybrid" (both)
	PriorityModel string `yaml:"priority_model"`

	// BlockedBehavior when a change's stage is "blocked".
	// "skip" (don't dispatch), "wait_with_timeout" (future), "escalate" (future)
	BlockedBehavior string `yaml:"blocked_behavior"`

	// AuditLog path for priority/stage/sync decisions. "" = stdout only.
	AuditLog string `yaml:"audit_log,omitempty"`

	// Metadata loading strategy.
	// "strict" (fail if metadata.json malformed), "lenient" (default to 0/backlog)
	MetadataStrategy string `yaml:"metadata_strategy"`
}

// SpecSyncBoard configuration for GitHub Projects board automation.
type SpecSyncBoard struct {
	// Enabled activates background board sync.
	Enabled bool `yaml:"enabled"`

	// SyncInterval how often to sync board state in background (e.g., "5m").
	SyncInterval time.Duration `yaml:"sync_interval"`

	// AutoSyncOnMetadataChange triggers immediate sync when .specsync/metadata.json changes.
	// Useful for real-time updates when priority/stage changes.
	AutoSyncOnMetadataChange bool `yaml:"auto_sync_on_metadata_change"`

	// ConflictStrategy when board and local disagree.
	// "report" (log and skip), "prompt_human" (future), "favor_local" (push local)
	ConflictStrategy string `yaml:"conflict_strategy"`

	// MaxRetries for failed syncs before giving up.
	MaxRetries int `yaml:"max_retries"`
}

// SpecSyncDispatcher configuration for priority-based change selection.
type SpecSyncDispatcher struct {
	// Enabled activates priority-aware dispatch (otherwise FIFO).
	Enabled bool `yaml:"enabled"`

	// IncludeActiveInQueue if true, "active" stage changes can be re-assigned.
	// If false (default), only "backlog" can be picked.
	IncludeActiveInQueue bool `yaml:"include_active_in_queue"`

	// SecondarySort tie-breaker for same priority.
	// "creation_date" (older first), "slug" (alphabetical)
	SecondarySort string `yaml:"secondary_sort"`

	// MinPriority to consider (skip below this). 0 = no floor.
	MinPriority int `yaml:"min_priority"`

	// AllowNegativePriority if true, allow negative priority values (reserved future use).
	AllowNegativePriority bool `yaml:"allow_negative_priority"`
}

// Validate checks SpecSync configuration for errors.
func (c *SpecSyncConfig) Validate() error {
	if !c.Enabled {
		return nil // Validation not needed if disabled
	}

	// Priority model validation
	if c.PriorityModel != "" {
		switch c.PriorityModel {
		case "specsync", "p1-p5", "hybrid":
			// OK
		default:
			return fmt.Errorf("invalid priority_model %q; must be specsync, p1-p5, or hybrid", c.PriorityModel)
		}
	}

	// Blocked behavior validation
	if c.BlockedBehavior != "" {
		switch c.BlockedBehavior {
		case "skip", "wait_with_timeout", "escalate":
			// OK
		default:
			return fmt.Errorf("invalid blocked_behavior %q; must be skip, wait_with_timeout, or escalate", c.BlockedBehavior)
		}
	}

	// Metadata strategy validation
	if c.MetadataStrategy != "" {
		switch c.MetadataStrategy {
		case "strict", "lenient":
			// OK
		default:
			return fmt.Errorf("invalid metadata_strategy %q; must be strict or lenient", c.MetadataStrategy)
		}
	}

	// Board config validation
	if c.Board != nil && c.Board.Enabled {
		if err := c.Board.Validate(); err != nil {
			return fmt.Errorf("board config invalid: %w", err)
		}
	}

	// Dispatcher config validation
	if c.Dispatcher != nil && c.Dispatcher.Enabled {
		if err := c.Dispatcher.Validate(); err != nil {
			return fmt.Errorf("dispatcher config invalid: %w", err)
		}
	}

	return nil
}

// Validate checks board configuration.
func (b *SpecSyncBoard) Validate() error {
	if b.SyncInterval <= 0 {
		return fmt.Errorf("sync_interval must be > 0 (e.g., 5m)")
	}

	if b.SyncInterval < 1*time.Minute {
		return fmt.Errorf("sync_interval < 1m risks excessive API calls")
	}

	switch b.ConflictStrategy {
	case "report", "prompt_human", "favor_local":
		// OK
	default:
		return fmt.Errorf("invalid conflict_strategy %q; must be report, prompt_human, or favor_local", b.ConflictStrategy)
	}

	if b.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be >= 0")
	}

	return nil
}

// Validate checks dispatcher configuration.
func (d *SpecSyncDispatcher) Validate() error {
	switch d.SecondarySort {
	case "creation_date", "slug":
		// OK
	default:
		return fmt.Errorf("invalid secondary_sort %q; must be creation_date or slug", d.SecondarySort)
	}

	if d.MinPriority < 0 && !d.AllowNegativePriority {
		return fmt.Errorf("min_priority < 0 but allow_negative_priority is false")
	}

	return nil
}

// DefaultSpecSyncConfig returns sensible defaults.
func DefaultSpecSyncConfig() *SpecSyncConfig {
	return &SpecSyncConfig{
		Enabled:          true,
		PriorityModel:    "specsync",
		BlockedBehavior:  "skip",
		MetadataStrategy: "lenient",
		Board: &SpecSyncBoard{
			Enabled:                  true,
			SyncInterval:             5 * time.Minute,
			AutoSyncOnMetadataChange: true,
			ConflictStrategy:         "report",
			MaxRetries:               3,
		},
		Dispatcher: &SpecSyncDispatcher{
			Enabled:               true,
			IncludeActiveInQueue:  false,
			SecondarySort:         "creation_date",
			MinPriority:           0,
			AllowNegativePriority: false,
		},
	}
}

// Merge applies overrides from user config to defaults.
func (c *SpecSyncConfig) Merge(user *SpecSyncConfig) *SpecSyncConfig {
	if user == nil {
		return c
	}

	result := *c

	if user.Enabled != c.Enabled {
		result.Enabled = user.Enabled
	}

	if user.PriorityModel != "" {
		result.PriorityModel = user.PriorityModel
	}

	if user.BlockedBehavior != "" {
		result.BlockedBehavior = user.BlockedBehavior
	}

	if user.AuditLog != "" {
		result.AuditLog = user.AuditLog
	}

	if user.MetadataStrategy != "" {
		result.MetadataStrategy = user.MetadataStrategy
	}

	if user.Board != nil {
		if result.Board == nil {
			result.Board = user.Board
		} else {
			result.Board = c.Board.Merge(user.Board)
		}
	}

	if user.Dispatcher != nil {
		if result.Dispatcher == nil {
			result.Dispatcher = user.Dispatcher
		} else {
			result.Dispatcher = c.Dispatcher.Merge(user.Dispatcher)
		}
	}

	return &result
}

// Merge applies overrides to board config.
func (b *SpecSyncBoard) Merge(user *SpecSyncBoard) *SpecSyncBoard {
	if user == nil {
		return b
	}

	result := *b

	if user.Enabled != b.Enabled {
		result.Enabled = user.Enabled
	}

	if user.SyncInterval > 0 {
		result.SyncInterval = user.SyncInterval
	}

	if user.AutoSyncOnMetadataChange != b.AutoSyncOnMetadataChange {
		result.AutoSyncOnMetadataChange = user.AutoSyncOnMetadataChange
	}

	if user.ConflictStrategy != "" {
		result.ConflictStrategy = user.ConflictStrategy
	}

	if user.MaxRetries > 0 {
		result.MaxRetries = user.MaxRetries
	}

	return &result
}

// Merge applies overrides to dispatcher config.
func (d *SpecSyncDispatcher) Merge(user *SpecSyncDispatcher) *SpecSyncDispatcher {
	if user == nil {
		return d
	}

	result := *d

	if user.Enabled != d.Enabled {
		result.Enabled = user.Enabled
	}

	if user.IncludeActiveInQueue != d.IncludeActiveInQueue {
		result.IncludeActiveInQueue = user.IncludeActiveInQueue
	}

	if user.SecondarySort != "" {
		result.SecondarySort = user.SecondarySort
	}

	if user.MinPriority > 0 {
		result.MinPriority = user.MinPriority
	}

	if user.AllowNegativePriority != d.AllowNegativePriority {
		result.AllowNegativePriority = user.AllowNegativePriority
	}

	return &result
}
