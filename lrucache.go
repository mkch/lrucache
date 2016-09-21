// Package lrucache implements a thread safe LRU(Least Recently Used) cache that holds a limited number of values.
// Each time a value is accessed, it is moved to the head of a queue. When a value is added to a full cache, the value at the end of that queue is evicted.
package lrucache

import (
	"container/list"
	"sync"
)

// EntryRemoved is the function called for entries that have been removed.
// newValue is the new value which replaced the old one, if any.
type EntryRemoved func(key, oldValue, newValue interface{})

// CreateEntry is the function computes the value and entry size for the key.
// Called by GetEnsure to compute a cache miss
type CreateEntry func(key interface{}) (value interface{}, size uint)

type entry struct {
	k, v interface{}
	size uint
}

type LruCache struct {
	m            map[interface{}]*list.Element
	l            *list.List
	maxSize      uint
	size         uint
	entryRemoved EntryRemoved
	mutex        sync.RWMutex
}

// New creates a LRU cache.
// maxSize is the maximum size of the cache, aka the sum of entry sizes passed in PutSize and returned by CreateEntry.
// entryRemoved is a callback function which is called every time an entry was removed.
func New(maxSize uint, entryRemoved EntryRemoved) *LruCache {
	if maxSize == 0 {
		panic("Invalid cache size")
	}
	return &LruCache{m: make(map[interface{}]*list.Element), l: list.New(), maxSize: maxSize, entryRemoved: entryRemoved}
}

// MaxSize returns the the maximum size of the cache. See New.
func (cache *LruCache) MaxSize() uint {
	return cache.maxSize
}

// Size returns the current size of the cache.
func (cache *LruCache) Size() uint {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	return cache.size
}

// Get returns the value for key or nil if no value is found.
// If a value was returned, it is moved to the head of the queue.
func (cache *LruCache) Get(key interface{}) (value interface{}) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	var element *list.Element
	if element = cache.m[key]; element != nil {
		value = element.Value.(*entry).v
		cache.l.MoveBefore(element, cache.l.Front())
	}
	return
}

// GetEnsure does similar work as Get except it creates the value, and moves it to the head of the queue, if not found.
func (cache *LruCache) GetEnsure(key interface{}, create CreateEntry) (value interface{}) {
	if value = cache.Get(key); value != nil {
		return
	}

	var size uint
	// This may take a long time, and the map may be different when create() returns
	value, size = create(key)

	cache.mutex.Lock()
	if winner, ok := cache.m[key]; ok {
		// This goroutine failed in the race. Discard.
		cache.mutex.Unlock()
		value = winner
		if cache.entryRemoved != nil {
			cache.entryRemoved(key, value, nil)
		}
	} else {
		var oldValue interface{}
		var evicted []*entry
		if winner, ok := cache.m[key]; ok {
			value = winner
			if cache.entryRemoved != nil {
				cache.entryRemoved(key, value, nil)
			}
		} else {
			oldValue, evicted = cache.putSize(key, value, size)
		}
		cache.mutex.Unlock()

		if cache.entryRemoved != nil {
			if oldValue != nil {
				cache.entryRemoved(key, oldValue, value)
			}
			for _, toEvict := range evicted {
				cache.entryRemoved(toEvict.k, toEvict.v, nil)
			}
		}
	}
	return
}

func (cache *LruCache) putSize(key, value interface{}, size uint) (oldValue interface{}, evicted []*entry) {
	if value == nil {
		panic("nil value")
	}
	if element, exists := cache.m[key]; exists {
		// Relpace the old value of existing entry.
		entry := element.Value.(*entry)
		oldValue = entry.v
		entry.v = value
		oldSize := entry.size
		entry.size = size
		cache.size -= oldSize
		cache.size += size
		// Move the element
		cache.l.MoveBefore(element, cache.l.Front())
	} else {
		// Add a new entry.
		newEntry := &entry{k: key, v: value, size: size}
		cache.size += size
		cache.m[key] = cache.l.PushFront(newEntry)
		// Trim
		for cache.size > cache.maxSize {
			eledst := cache.l.Back()
			cache.l.Remove(eledst)
			toEvict := eledst.Value.(*entry)
			delete(cache.m, toEvict.k)
			cache.size -= toEvict.size
			evicted = append(evicted, &entry{k: toEvict.k, v: toEvict.v, size: toEvict.size})
		}
	}
	return
}

// PutSize caches value for key and moves this entry to the head of the queue. size is the entry size.
// The return value oldValue, if not nil, is the old value replaced by value(no new entry was added).
// The non-nil EntryRemoved function passed in New() is called when an old value was replaced
// or the last entry in the queue was evicted to make space.
func (cache *LruCache) PutSize(key, value interface{}, size uint) (oldValue interface{}) {
	var evicted []*entry
	cache.mutex.Lock()
	oldValue, evicted = cache.putSize(key, value, size)
	cache.mutex.Unlock()
	if cache.entryRemoved != nil {
		if oldValue != nil {
			cache.entryRemoved(key, oldValue, value)
		}
		for _, toEvict := range evicted {
			cache.entryRemoved(toEvict.k, toEvict.v, nil)
		}
	}
	return
}

// Put calls PutSize(key, value, 1)
func (cache *LruCache) Put(key, value interface{}) (oldValue interface{}) {
	return cache.PutSize(key, value, 1)
}

// Remove removes the entry for key. Returns the value for key if exists, or nil otherwise.
// The non-nil EntryRemoved function passed in New() is called when an entry was actually removed.
func (cache *LruCache) Remove(key interface{}) (value interface{}) {
	cache.mutex.Lock()
	var element *list.Element
	var k, v interface{}
	if element = cache.m[key]; element != nil {
		delete(cache.m, key)
		entry := cache.l.Remove(element).(*entry)
		value = entry.v
		cache.size -= entry.size
		k = entry.k
		v = entry.v
	}
	cache.mutex.Unlock()

	if v != nil && cache.entryRemoved != nil {
		cache.entryRemoved(k, v, nil)
	}
	return
}
