package storage

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

const gcBatchSize = 256
const GCDetailLimit = 1000

type gcCandidate struct {
	path string
	info os.FileInfo
}

// walkFiles reads directories incrementally, including directories containing
// many obsolete immutable versions; it never follows symbolic links.
func walkFiles(root string, skipMultipart bool, visit func(string, os.FileInfo) error) error {
	dir, err := os.Open(root)
	if err != nil {
		return err
	}
	defer dir.Close()
	for {
		entries, readErr := dir.ReadDir(gcBatchSize)
		for _, entry := range entries {
			path := filepath.Join(root, entry.Name())
			if entry.IsDir() {
				if skipMultipart && entry.Name() == ".multipart" {
					continue
				}
				if err := walkFiles(path, skipMultipart, visit); err != nil {
					return err
				}
			} else {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if err := visit(path, info); err != nil {
					return err
				}
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// Membership is resolved in indexed batches, not a map of all live objects.
func (f *FileStore) scanObjectOrphans(metadata *MetadataStore, visit func(OrphanFile) error) error {
	batch := make([]gcCandidate, 0, gcBatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		args := make([]interface{}, len(batch))
		for i, item := range batch {
			args[i] = item.path
		}
		query := "SELECT storage_path FROM objects WHERE storage_path IN (" + strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",") + ")"
		rows, err := metadata.db.Query(query, args...)
		if err != nil {
			return err
		}
		known := make(map[string]bool, len(batch))
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				rows.Close()
				return err
			}
			known[path] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, item := range batch {
			if !known[item.path] {
				rel, err := filepath.Rel(f.basePath, item.path)
				if err != nil {
					return err
				}
				if err := visit(OrphanFile{Path: rel, Size: item.info.Size(), ModifiedAt: item.info.ModTime()}); err != nil {
					return err
				}
			}
		}
		batch = batch[:0]
		return nil
	}
	err := walkFiles(f.basePath, true, func(path string, info os.FileInfo) error {
		batch = append(batch, gcCandidate{path, info})
		if len(batch) == gcBatchSize {
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	return flush()
}

func (f *FileStore) scanMultipartOrphans(metadata *MetadataStore, visit func(OrphanFile) error) error {
	root := filepath.Join(f.basePath, ".multipart")
	dir, err := os.Open(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer dir.Close()
	for {
		entries, readErr := dir.ReadDir(gcBatchSize)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			upload, err := metadata.GetMultipartUpload(entry.Name())
			if err != nil {
				return err
			}
			if upload != nil {
				continue
			}
			err = walkFiles(filepath.Join(root, entry.Name()), false, func(path string, info os.FileInfo) error {
				rel, err := filepath.Rel(f.basePath, path)
				if err != nil {
					return err
				}
				return visit(OrphanFile{Path: rel, Size: info.Size(), ModifiedAt: info.ModTime()})
			})
			if err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// Remove directories only after their handles are closed, including on Windows.
// Live files keep their ancestor directories; the store root is always retained.
func pruneEmptyGCDirectories(root string, removeRoot bool) (bool, error) {
	dir, err := os.Open(root)
	if err != nil {
		return false, err
	}
	empty := true
	for {
		entries, readErr := dir.ReadDir(gcBatchSize)
		for _, entry := range entries {
			if !entry.IsDir() {
				empty = false
				continue
			}
			removed, err := pruneEmptyGCDirectories(filepath.Join(root, entry.Name()), true)
			if err != nil {
				dir.Close()
				return false, err
			}
			if !removed {
				empty = false
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			dir.Close()
			return false, readErr
		}
	}
	if err := dir.Close(); err != nil {
		return false, err
	}
	if !empty || !removeRoot {
		return false, nil
	}
	if err := os.Remove(root); err != nil {
		return false, err
	}
	return true, nil
}
