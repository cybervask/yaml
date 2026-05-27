package yaml

import (
	"sync"
)

// includeEntry holds metadata about a YAML !include directive.
type includeEntry struct {
	// RelativePath is the path as written in the YAML source (e.g., "configs/server/logging.yaml").
	RelativePath string
	// AbsolutePath is the resolved absolute path on the filesystem (e.g., "/etc/app/configs/server/logging.yaml").
	AbsolutePath string
}

var (
	includeMu  sync.RWMutex
	includeMap = make(map[string]includeEntry)
)

// RegisterIncludePath records the mapping between a YAML field path and its included file.
// It is called by the parser when processing a !include directive.
//
// yamlPath is the dot-separated path to the field in the configuration structure (e.g., "server.logging").
// relativePath is the path as written in the YAML source.
// absolutePath is the fully resolved filesystem path to the included file.
func RegisterIncludePath(yamlPath, relativePath, absolutePath string) {
	includeMu.Lock()
	defer includeMu.Unlock()
	includeMap[yamlPath] = includeEntry{
		RelativePath: relativePath,
		AbsolutePath: absolutePath,
	}
}

// FindIncludeFile returns the relative path as written in the original YAML source.
//
// Example: FindIncludeFile("server.logging") returns "configs/server/logging.yaml".
// Returns an empty string if the path was not loaded via a !include directive.
func FindIncludeFile(yamlPath string) string {
	includeMu.RLock()
	defer includeMu.RUnlock()
	if entry, ok := includeMap[yamlPath]; ok {
		return entry.RelativePath
	}
	return ""
}

// FindIncludeFileAbs returns the absolute filesystem path to the included file.
//
// Example: FindIncludeFileAbs("server.logging") returns "/etc/app/configs/server/logging.yaml".
// Returns an empty string if the path was not loaded via a !include directive.
func FindIncludeFileAbs(yamlPath string) string {
	includeMu.RLock()
	defer includeMu.RUnlock()
	if entry, ok := includeMap[yamlPath]; ok {
		return entry.AbsolutePath
	}
	return ""
}

// ResetIncludeTracker clears all tracked include paths.
// It should be called at the beginning of each load operation (Load, UnmarshalFile).
func ResetIncludeTracker() {
	includeMu.Lock()
	defer includeMu.Unlock()
	includeMap = make(map[string]includeEntry)
}
