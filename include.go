package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	yaml4 "go.yaml.in/yaml/v4"
)

// maxIncludeDepth bounds the !include resolution depth. It acts as a hard
// safety net behind cycle detection: pathological chains (including ones
// obscured by symlink loops that defeat path comparison) terminate with a
// configuration error instead of exhausting memory.
const maxIncludeDepth = 32

// includeChain tracks the absolute paths of the include files currently being
// resolved, from the outermost to the innermost. Its immutable-push design
// keeps sibling branches of the resolution tree isolated from each other.
type includeChain struct {
	parents map[string]struct{}
	depth   int
}

// newIncludeChain returns the empty root chain.
func newIncludeChain() includeChain {
	return includeChain{parents: make(map[string]struct{})}
}

// with returns a chain extended with path. The receiver is not modified:
// sibling subtrees share the parent chain without observing each other's files.
func (c includeChain) with(path string) includeChain {
	next := includeChain{parents: make(map[string]struct{}, len(c.parents)+1), depth: c.depth + 1}
	for p := range c.parents {
		next.parents[p] = struct{}{}
	}

	next.parents[path] = struct{}{}

	return next
}

// contains reports whether path is already being resolved higher up the chain.
func (c includeChain) contains(path string) bool {
	_, found := c.parents[path]

	return found
}

// resolveIncludes recursively traverses a YAML AST and replaces !include scalar nodes
// with the parsed content from external files.
//
// node is the current AST node being processed.
// currentDir is the base directory for resolving relative include paths.
// yamlPathPrefix is the dot-separated path to the current field in the configuration structure.
// chain tracks the ancestor include files for cycle detection.
// session receives the include metadata of the object being loaded; it may be
// nil for load paths that do not bind include data to an object.
//
// Include metadata is recorded per include level: a marker directly in the
// main config (level 1 — including a marker that occupies the whole document,
// recorded under the empty path) gets a session entry for write-back; a
// marker inside an included file (level 2+) is recorded as nested-only, so
// its file is never written back and the subtree carrying it is left
// untouched by dumps.
func resolveIncludes(node *yaml4.Node, currentDir, yamlPathPrefix string, chain includeChain, session *includeSession) error {
	if node.Tag == "!include" && node.Kind == yaml4.ScalarNode {
		originalPath := node.Value

		fullPath := originalPath
		if !filepath.IsAbs(originalPath) {
			fullPath = filepath.Join(currentDir, originalPath)
		}
		// Normalize absolute spellings too: Join already cleans relative
		// paths, but an absolute path written with redundant elements
		// (/d/./a.yaml) must resolve to the same cycle-detection key,
		// tracker value and write-back path as its cleaned spelling.
		// Clean is idempotent for already-normalized paths.
		fullPath = filepath.Clean(fullPath)

		// Cycle guard: an include file that is already being resolved higher up
		// the chain means the configuration graph is cyclic.
		if chain.contains(fullPath) {
			return fmt.Errorf("cyclic !include detected: %s is already being resolved", fullPath)
		}

		// Depth guard: a hard stop behind cycle detection that also covers
		// symlink loops and unboundedly long chains.
		if chain.depth >= maxIncludeDepth {
			return fmt.Errorf("maximum !include depth %d exceeded at %s", maxIncludeDepth, fullPath)
		}

		includeData, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read included file %s: %w", fullPath, err)
		}

		// The include file must hold exactly one YAML document. The document
		// count is verified by streaming the bytes through a dedicated loader:
		// yaml4 keeps only the first document in a Node, so a plain Unmarshal
		// would silently drop every document after the first one.
		docLoader, err := yaml4.NewLoader(bytes.NewReader(includeData))
		if err != nil {
			return fmt.Errorf("failed to parse included file %s: %w", fullPath, err)
		}

		var doc yaml4.Node
		if err := docLoader.Load(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("included file %s contains no documents", fullPath)
			}
			return fmt.Errorf("failed to parse included file %s: %w", fullPath, err)
		}

		var discard yaml4.Node
		switch err := docLoader.Load(&discard); {
		case err == nil:
			// The second Load returned a document: the include file is
			// multi-document, which the include mechanism does not support.
			return fmt.Errorf("included file %s contains multiple documents", fullPath)
		case errors.Is(err, io.EOF):
			// Exactly one document — the supported shape.
		default:
			return fmt.Errorf("failed to parse included file %s: %w", fullPath, err)
		}

		if len(doc.Content) == 0 {
			return fmt.Errorf("included file %s contains no documents", fullPath)
		}
		actualContentNode := doc.Content[0]

		// The include level counts the !include hops from the main config.
		// Level-1 markers — directly in the main config, including a marker
		// that occupies the whole document (tracked under the empty path) —
		// receive session entries and are written back by DumpWithInclude.
		// Nested markers (level 2+) never get write-back entries; they are
		// recorded so the dump can leave the subtree carrying them untouched
		// and reject fields that hold several markers at once.
		level := chain.depth + 1
		if level == 1 {
			session.record(yamlPathPrefix, includeEntry{RelativePath: originalPath, AbsolutePath: fullPath}, level)
		} else {
			session.recordNested(yamlPathPrefix, level)
		}
		if yamlPathPrefix != "" {
			RegisterIncludePath(yamlPathPrefix, originalPath, fullPath)
		}

		newDir := filepath.Dir(fullPath)
		if err := resolveIncludes(actualContentNode, newDir, yamlPathPrefix, chain.with(fullPath), session); err != nil {
			return err
		}

		*node = *actualContentNode
		return nil
	}

	if node.Kind == yaml4.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			key := keyNode.Value

			nextPath := key
			if yamlPathPrefix != "" {
				nextPath = yamlPathPrefix + "." + key
			}

			if err := resolveIncludes(valNode, currentDir, nextPath, chain, session); err != nil {
				return err
			}
		}
	} else {
		for _, child := range node.Content {
			if err := resolveIncludes(child, currentDir, yamlPathPrefix, chain, session); err != nil {
				return err
			}
		}
	}

	return nil
}
