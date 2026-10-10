package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkAuditGCManyObjects(b *testing.B) {
	for _, count := range []int{2000, 20000} {
		b.Run(fmt.Sprintf("objects_%d", count), func(b *testing.B) {
			root := b.TempDir()
			fs, err := NewFileStore(filepath.Join(root, "blobs"))
			if err != nil {
				b.Fatal(err)
			}
			ms, err := NewMetadataStore(filepath.Join(root, "metadata.db"))
			if err != nil {
				b.Fatal(err)
			}
			defer ms.Close()
			if err := ms.CreateBucket("benchmark"); err != nil {
				b.Fatal(err)
			}
			tx, err := ms.db.Begin()
			if err != nil {
				b.Fatal(err)
			}
			stmt, err := tx.Prepare("INSERT INTO objects(bucket,key,size,etag,content_type,last_modified,storage_path) VALUES('benchmark',?,1,'etag','text/plain',?,?)")
			if err != nil {
				b.Fatal(err)
			}
			for i := 0; i < count; i++ {
				key := fmt.Sprintf("key-%08d", i)
				path := fs.GetStoragePath("benchmark", key)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					b.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
					b.Fatal(err)
				}
				if i%20 != 0 {
					if _, err := stmt.Exec(key, time.Now(), path); err != nil {
						b.Fatal(err)
					}
				}
			}
			stmt.Close()
			if err := tx.Commit(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := RunGC(fs, ms, time.Hour, true)
				if err != nil || result.OrphanCount != count/20 {
					b.Fatal(result, err)
				}
			}
		})
	}
}
