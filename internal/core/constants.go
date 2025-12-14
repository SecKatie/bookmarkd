package core

import "time"

// Archive status values used in database and handlers
const (
	ArchiveStatusOK    = "ok"
	ArchiveStatusError = "error"
)

// Timeout defaults for archiving operations
const (
	DefaultArchiveTimeout   = 35 * time.Second
	DefaultResourceTimeout  = 10 * time.Second
	DefaultNetworkIdleDelay = 500 * time.Millisecond
)

// Resource limits
const (
	MaxResourceSize = 5 * 1024 * 1024 // 5MB
)

// HTTP client configuration
const (
	UserAgent = "Mozilla/5.0 (compatible; bookmarkd/1.0)"
)
