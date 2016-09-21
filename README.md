# lrucache
Go implementation of LRU(Least Recently Used) cache.

```
cache := lrucache.New(10, nil)
// Cache a value.
cache.Put(key, value)
// Query the cached value.
if cachedValue := cache.Get(somekey); value != nil {
    doStuff(value)
}
```
