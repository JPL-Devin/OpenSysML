package libs

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

// Cache persists reduced library IndexRecords to disk keyed by content hash and
// format version. A hit lets the loader skip lexing/parsing entirely. The same
// mechanism serves git dependencies (Plan 5c) since it keys purely on content.
type Cache struct {
	dir string
}

// NewCache resolves the cache directory ($XDG_CACHE_HOME or os.UserCacheDir),
// appends "sysml-ls/libs", creates it, and returns a ready Cache.
func NewCache() (*Cache, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		d, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		base = d
	}
	dir := filepath.Join(base, "sysml-ls", "libs")
	// #nosec G703 -- the base directory is the user's own XDG_CACHE_HOME or OS
	// cache directory, and the joined suffix is a constant.
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Cache{dir: dir}, nil
}

// keyFor derives a cache key from the file content and the current format
// version. Any content change or version bump yields a distinct key, so stale
// entries are simply never found (miss) rather than requiring explicit
// invalidation.
func (c *Cache) keyFor(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]) + "-v" + strconv.Itoa(formatVersion)
}

func (c *Cache) path(key string) string {
	return filepath.Join(c.dir, key+".idx")
}

// Load returns the cached record for key, or (nil, false) on any miss
// (absent file, read error, or decode error — all treated as a benign miss).
func (c *Cache) Load(key string) (*IndexRecord, bool) {
	// #nosec G304 G703 -- the path is <cache dir>/<content hash>.idx; the key is
	// hex-encoded SHA-256 computed by keyFor, never caller-supplied text.
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return nil, false
	}
	var rec IndexRecord
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&rec); err != nil {
		return nil, false
	}
	return &rec, true
}

// Store gob-encodes rec and writes it to <dir>/<key>.idx atomically: it writes a
// sibling <key>.idx.tmp then renames it into place, so a concurrent Load or a
// crash never observes a partially written file. A failed encode/write removes
// the temp and leaves any existing final file untouched.
func (c *Cache) Store(key string, rec *IndexRecord) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rec); err != nil {
		return err
	}
	final := c.path(key)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		if rmErr := os.Remove(tmp); rmErr != nil {
			return errors.Join(err, rmErr)
		}
		return err
	}
	return nil
}
