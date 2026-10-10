package storage

import "sync"

type objectLock struct {
	mu    sync.Mutex
	users int
}

// LockObject serializes operations on one logical object. Callers hold
// MutationMu.RLock first, so maintenance/bucket deletion cannot invalidate a
// publication or collect a blob while a reader or writer is using it.
// Entries are removed at the last release; the map does not grow with key count.
func (f *FileStore) LockObject(bucket, key string) func() {
	name := bucket + "\x00" + key
	f.locksMu.Lock()
	if f.objectLocks == nil {
		f.objectLocks = make(map[string]*objectLock)
	}
	lock := f.objectLocks[name]
	if lock == nil {
		lock = &objectLock{}
		f.objectLocks[name] = lock
	}
	lock.users++
	f.locksMu.Unlock()
	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		f.locksMu.Lock()
		lock.users--
		if lock.users == 0 {
			delete(f.objectLocks, name)
		}
		f.locksMu.Unlock()
	}
}
