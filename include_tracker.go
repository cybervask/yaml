package yaml

import (
	"reflect"
	"strings"
	"sync"
)

// includeEntry holds metadata about a YAML !include directive.
type includeEntry struct {
	// RelativePath is the path as written in the YAML source (e.g., "configs/server/logging.yaml").
	RelativePath string
	// AbsolutePath is the resolved absolute path on the filesystem (e.g., "/etc/app/configs/server/logging.yaml").
	AbsolutePath string
}

// maxIncludeSessions bounds the number of retained include sessions. When the
// bound is exceeded, the least recently used session is evicted and its object
// falls back to the inline safe mode of DumpWithInclude.
const maxIncludeSessions = 128

// includeSession holds the include metadata recorded while loading a single
// configuration object. It is keyed by the address of the target pointer and
// keeps a strong reference to the object: the reference keeps the object
// alive, so the garbage collector cannot recycle its address and let a newly
// allocated object silently inherit a foreign session.
type includeSession struct {
	// owner is the strong reference to the loaded object the session belongs to.
	owner any
	// key is the address of the loaded object's pointer; it is the registry key.
	key uintptr
	// entries maps a dot-separated YAML field path to its include metadata.
	// Only !include markers that sit directly in the main config get entries
	// (a whole-document root include is stored under the empty path); the map
	// is filled during !include resolution, before the session becomes
	// visible to dumps, and is never mutated afterwards.
	entries map[string]includeEntry
	// levels records the include level of the first registration of every
	// marker path: 1 for a marker in the main config, 2+ for a marker inside
	// an included file.
	levels map[string]int
	// nested records the paths of markers found inside included files
	// (level 2+). Their files are never written back; the set lets a dump
	// freeze every subtree whose file carries nested markers.
	nested map[string]bool
	// multi records paths that carried more than one !include marker at the
	// same include level — e.g. "servers: [!include a.yaml, !include b.yaml]".
	// Such a field has no single marker to write back.
	multi map[string]bool
}

// newIncludeSession binds an empty session to out. It returns nil when out is
// not a non-nil pointer: only pointer targets get a session.
func newIncludeSession(out any) *includeSession {
	key, ok := sessionKey(out)
	if !ok {
		return nil
	}

	return &includeSession{
		owner:   out,
		key:     key,
		entries: make(map[string]includeEntry),
		levels:  make(map[string]int),
		nested:  make(map[string]bool),
		multi:   make(map[string]bool),
	}
}

// record stores the include metadata for yamlPath at the given include level
// (1 = marker in the main config, 2+ = marker inside an included file).
// Registration is first-wins, mirroring the global tracker: the registration
// closest to the main config wins over nested re-registrations of the same
// path. Re-registering a path at the level of its first registration means
// the field carried several markers; it is flagged so the dump can reject it
// instead of silently writing one marker back.
func (s *includeSession) record(yamlPath string, entry includeEntry, level int) {
	if s == nil {
		return
	}
	if prev, exists := s.levels[yamlPath]; exists {
		if prev == level {
			s.multi[yamlPath] = true
		}
		return
	}

	s.levels[yamlPath] = level
	s.entries[yamlPath] = entry
}

// recordNested registers a !include marker found inside an included file
// (level 2+). Such a marker never gets a session entry — its file is never
// written back — but the path is remembered: a dump leaves every file whose
// subtree carries nested markers untouched, and flags a field that repeated
// the path at the same level as multi-marker.
func (s *includeSession) recordNested(yamlPath string, level int) {
	if s == nil {
		return
	}
	s.nested[yamlPath] = true
	if prev, exists := s.levels[yamlPath]; exists {
		if prev == level {
			s.multi[yamlPath] = true
		}
		return
	}

	s.levels[yamlPath] = level
}

// hasNestedUnder reports whether any nested (level 2+) marker was recorded
// strictly below yamlPath — the include file behind yamlPath itself carries
// !include markers, so a write-back of the Go value would erase them. A
// marker collapsing onto the same path (a root marker inside the referenced
// file) does not count: its resolution merges into the outer file's value.
func (s *includeSession) hasNestedUnder(yamlPath string) bool {
	if s == nil {
		return false
	}
	for path := range s.nested {
		if path == yamlPath {
			continue
		}
		if yamlPath == "" || strings.HasPrefix(path, yamlPath+".") {
			return true
		}
	}

	return false
}

// isMulti reports whether yamlPath carried several !include markers at the
// same include level (e.g. a sequence of includes under one field).
func (s *includeSession) isMulti(yamlPath string) bool {
	if s == nil {
		return false
	}

	return s.multi[yamlPath]
}

// entry returns the include metadata recorded for yamlPath. A nil session
// matches nothing, which drives the inline safe mode of DumpWithInclude.
func (s *includeSession) entry(yamlPath string) (includeEntry, bool) {
	if s == nil {
		return includeEntry{}, false
	}

	entry, ok := s.entries[yamlPath]

	return entry, ok
}

// sessionKey returns the registry key for out: the address of the pointer
// value it holds. It reports false when out is not a non-nil pointer.
func sessionKey(out any) (uintptr, bool) {
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return 0, false
	}

	return v.Pointer(), true
}

// includeRegistry caches the global mapping between configuration field paths
// and their !include file metadata. Access is guarded by includeMu.
var (
	includeMu  sync.RWMutex
	includeMap = make(map[string]includeEntry)
	// includeSessions holds the per-object include sessions keyed by the
	// address of the loaded object's pointer. sessionOrder keeps the keys in
	// least-recently-used order for eviction under maxIncludeSessions.
	includeSessions = make(map[uintptr]*includeSession)
	sessionOrder    []uintptr
)

// commitIncludeSession registers a fully resolved session in the per-object
// registry. Loading the same pointer again replaces its session, so the dump
// of the object reflects the most recent load.
func commitIncludeSession(s *includeSession) {
	if s == nil {
		return
	}

	includeMu.Lock()
	defer includeMu.Unlock()

	if _, exists := includeSessions[s.key]; exists {
		touchSessionLocked(s.key)
	} else {
		sessionOrder = append(sessionOrder, s.key)
	}

	includeSessions[s.key] = s

	if len(sessionOrder) > maxIncludeSessions {
		evictOldestSessionLocked()
	}
}

// lookupIncludeSession returns the session registered for the pointer in out,
// or nil when out is not a non-nil pointer or no session is registered for it.
func lookupIncludeSession(out any) *includeSession {
	key, ok := sessionKey(out)
	if !ok {
		return nil
	}

	includeMu.Lock()
	defer includeMu.Unlock()

	s, found := includeSessions[key]
	if found {
		touchSessionLocked(key)
	}

	return s
}

// touchSessionLocked moves key to the most-recently-used end of sessionOrder.
func touchSessionLocked(key uintptr) {
	for i, k := range sessionOrder {
		if k == key {
			copy(sessionOrder[i:], sessionOrder[i+1:])
			sessionOrder[len(sessionOrder)-1] = key

			return
		}
	}
}

// evictOldestSessionLocked drops the least recently used session. The evicted
// object falls back to the inline safe mode of DumpWithInclude.
func evictOldestSessionLocked() {
	oldest := sessionOrder[0]
	copy(sessionOrder, sessionOrder[1:])
	sessionOrder = sessionOrder[:len(sessionOrder)-1]
	delete(includeSessions, oldest)
}

// dropIncludeSession removes the session registered for the pointer in out.
// The stream load paths (Loader.Load, Decoder.Decode) call it after a
// successful load of the same pointer: the stream replaced the object's
// content without binding a new session, so a session recorded by an earlier
// [Load]/[LoadFile] no longer reflects the object and must not drive a later
// write-back — otherwise values decoded from the stream would be written into
// the files recorded by the earlier load. A pointer without a session is a
// no-op, and a failed stream load keeps the session (the object is unchanged).
func dropIncludeSession(out any) {
	key, ok := sessionKey(out)
	if !ok {
		return
	}

	includeMu.Lock()
	defer includeMu.Unlock()

	if _, exists := includeSessions[key]; !exists {
		return
	}
	delete(includeSessions, key)
	for i, k := range sessionOrder {
		if k == key {
			sessionOrder = append(sessionOrder[:i], sessionOrder[i+1:]...)

			break
		}
	}
}

// RegisterIncludePath records the mapping between a YAML field path and its included file.
// It is called by the parser when processing a !include directive.
//
// Registration is first-wins: if yamlPath is already tracked in the current
// load cycle, the call is a no-op. This keeps the mapping of the field closest
// to the main config intact when the root of an included file is itself a
// !include directive that re-registers the same YAML path. Distinct YAML paths
// are registered as usual.
//
// yamlPath is the dot-separated path to the field in the configuration structure (e.g., "server.logging").
// relativePath is the path as written in the YAML source.
// absolutePath is the fully resolved filesystem path to the included file.
func RegisterIncludePath(yamlPath, relativePath, absolutePath string) {
	includeMu.Lock()
	defer includeMu.Unlock()
	if _, exists := includeMap[yamlPath]; exists {
		return
	}
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

// ResetIncludeTracker clears the globally tracked include paths of the current
// load cycle. It should be called at the beginning of each load operation
// (Load, LoadFile, Loader.Load, Decoder.Decode).
//
// The per-object include sessions are intentionally retained: they are bound
// to the loaded objects themselves, so the dump of an object loaded before
// the most recent load keeps writing back to its own include files instead of
// silently degrading to inline output. Sessions are retired only explicitly —
// by replacement on a re-load of the same pointer, by a successful stream load
// of the same pointer ([Loader.Load], [Decoder.Decode]; see dropIncludeSession)
// and by the maxIncludeSessions least-recently-used bound.
func ResetIncludeTracker() {
	includeMu.Lock()
	defer includeMu.Unlock()
	includeMap = make(map[string]includeEntry)
}
