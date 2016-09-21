package lrucache_test

import (
	"github.com/mkch/lrucache"
	"strconv"
	"sync"
	"testing"
)

func TestPutGet(t *testing.T) {
	cache := lrucache.New(10, nil)
	cache.Put(1, "1")
	cache.Put(2, "2")
	if size := cache.Size(); size != 2 {
		t.Fatalf("Wrong size. 2 expected, but %d returned.", size)
	}
	if value := cache.Get(1); value != "1" {
		t.Fatalf("Wrong value returned by LruCache.Get. \"1\" expected, \"%v\" returned", value)
	}
	if value := cache.Get(2); value != "2" {
		t.Fatalf("Wrong value returned by LruCache.Get. \"2\" expected, but \"%v\" returned", value)
	}
	if value := cache.Get(3); value != nil {
		t.Fatalf("Wrong value returned by LruCache.Get. nil expected, but \"%v\" returned", value)
	}
}

func TestGetEnsure(t *testing.T) {
	cache := lrucache.New(10, nil)
	cache.Put("key1", 100)
	create := func(key interface{}) (value interface{}, size uint) {
		switch key {
		case "key2":
			return "200", 1
		default:
			panic("Invalid key")
		}
	}
	if value := cache.GetEnsure("key2", create); value != "200" {
		t.Fatalf("Wrong value returned by LruCache.GetEnsure. \"200\" expected, but \"%v\" returned", value)
	}
	if value := cache.Get("key2"); value != "200" {
		t.Fatalf("Wrong value returned by LruCache.GetEnsure. \"200\" expected, but \"%v\" returned", value)
	}
	if value := cache.Get("key1"); value != 100 {
		t.Fatalf("Wrong value returned by LruCache.GetEnsure. 100 expected, but %v returned", value)
	}
}

func TestSize(t *testing.T) {
	cache := lrucache.New(5, nil)
	if size := cache.Size(); size != 0 {
		t.Fatal("Wrong size for empty cache")
	}
	cache.Put(1, 100.0)
	if size := cache.Size(); size != 1 {
		t.Fatalf("Wrong value returned by LruCache.Size. 1 expected, but %v returned", size)
	}
	cache.PutSize("2", 22, 3)
	if size := cache.Size(); size != 4 {
		t.Fatalf("Wrong value returned by LruCache.Size. 4 expected, but %v returned", size)
	}
	cache.PutSize(3, 300, 2) // Exceeds maxSize by 1, entry (1, 100.0) should be evicted.
	if size := cache.Size(); size != 5 {
		t.Fatalf("Wrong value returned by LruCache.Size. 5 expected, but %v returned", size)
	}
	if value := cache.Get(1); value != nil {
		t.Fatalf("Wrong value returned by LruCache.Get. nil expected, but %v returned", value)
	}
}

func TestRemove(t *testing.T) {
	cache := lrucache.New(5, nil)
	cache.PutSize(1, 100, 4)
	cache.Put(2, 200)
	cache.Remove(1)
	if size := cache.Size(); size != 1 {
		t.Fatalf("Wrong value returned by LruCache.Size. 1 expected, but %v returned", size)
	}
	if value := cache.Get(1); value != nil {
		t.Fatalf("Wrong value returned by LruCache.Get. nil expected, but \"%v\" returned", value)
	}
	if value := cache.Get(2); value != 200 {
		t.Fatalf("Wrong value returned by LruCache.Get. nil expected, but \"%v\" returned", value)
	}
}

func TestCallback(t *testing.T) {
	var fCalled bool
	var removalKey, removalOldValue, removalNewValue interface{}
	f := func(key, oldValue, newValue interface{}) {
		fCalled = true
		removalKey = key
		removalOldValue = oldValue
		removalNewValue = newValue
	}

	cache := lrucache.New(5, f)
	if fCalled {
		t.Fatal("Callback should not be called")
	}
	cache.Put("1", 1)
	cache.Put("2", 2)
	cache.Put("3", 3)
	if fCalled {
		t.Fatal("Callback should not be called")
	}
	cache.PutSize("4", "400", 3)
	if !fCalled || removalKey != "1" || removalOldValue != 1 || removalNewValue != nil {
		t.Fatalf("true, \"1\", 1, nil expected, but %v, \"%v\", %v, %v got", fCalled, removalKey, removalOldValue, removalNewValue)
	}
	fCalled = false
	removalKey = nil
	removalOldValue = nil
	removalNewValue = nil
	cache.Put("3", 30)
	if !fCalled || removalKey != "3" || removalOldValue != 3 || removalNewValue != 30 {
		t.Fatalf("true, \"3\", 3, 30, nil expected, but %v, \"%v\", %v, %v got", fCalled, removalKey, removalOldValue, removalNewValue)
	}
}

func TestConcurrent(t *testing.T) {
	cache := lrucache.New(20, nil)
	waitGroup := &sync.WaitGroup{}

	for i := 0; i < 100; i++ {
		cache.Put(i, strconv.Itoa(i))
	}

	for n := 0; n < 1000; n++ {
		waitGroup.Add(3)
		go func() {
			for i := 0; i < 100; i++ {
				cache.Remove(i)
			}
			waitGroup.Done()
		}()

		go func() {
			for i := 101; i < 200; i++ {
				str := strconv.Itoa(i)
				cache.Put(i, str)
			}
			waitGroup.Done()
		}()

		go func() {
			for i := 201; i < 300; i++ {
				str := strconv.Itoa(i)
				cache.Put(i, str)
			}
			waitGroup.Done()
		}()

		waitGroup.Wait()
	}

	for i := 0; i < 100; i++ {
		if value := cache.Get(i); value != nil {
			t.Fatalf("Wrong value returned by LruCache.Get. nil expected, but  \"%v\" returned", value)
		}
	}
	var remainCount uint
	for i := 101; i < 300; i++ {
		str := strconv.Itoa(i)
		if value := cache.Get(i); value != nil {
			remainCount++
			if value != str {
				t.Fatalf("Wrong value returned by LruCache.Get. \"%v\" expected, but \"%v\" returned", str, value)
			}
		}
	}
	if remainCount != cache.MaxSize() {
		t.Fatalf("Wrong remainCount. %v expected, but %v got", cache.MaxSize, remainCount)
	}
}

func BenchmarkPut(b *testing.B) {
	cache := lrucache.New(2000, nil)
	for i := 0; i < b.N; i++ {
		cache.Put(i%300, i)
	}
}

var cacheForBenchmarkGet = lrucache.New(2000, nil)

func init() {
	for i := 0; i < 1200; i++ {
		cacheForBenchmarkGet.Put(i, i+1)
	}
}

func BenchmarkGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cacheForBenchmarkGet.Get(i)
	}
}
