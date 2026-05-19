# go-cache

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-green.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/tunein/go-cache)](https://goreportcard.com/report/github.com/tunein/go-cache)

A high-performance, goroutine-safe, generic in-memory cache implementation for Go with automatic expiration, lazy loading, and duplicate function call suppression.

## Features

- 🚀 **Generic & Type-Safe**: Full Go generics support for any comparable key and value types
- 🔒 **Thread-Safe**: Can be accessed concurrently from multiple goroutines
- ⏰ **Automatic Expiration**: Configurable TTL with per-item override support
- 🔄 **Lazy Loading**: Automatic value loading when cache misses occur
- 🚫 **Duplicate Call Suppression**: Prevents multiple simultaneous calls for the same key
- 📊 **Statistics & Monitoring**: Built-in hooks for cache events
- 🎯 **LRU-like Behavior**: Efficient memory management

## Installation

```bash
go get github.com/tunein/go-cache
```

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    "github.com/tunein/go-cache"
)

func main() {
    // Create a new cache with 1 hour default TTL
    cc := cache.New[string, int](1 * time.Hour)
    
    // Set a value with default TTL
    cc.Set("awesome-key", 100)
    
    // Set a value with custom TTL (never expire)
    cc.SetWithExpire("decent-key", 200, 0)
    
    // Get a value
    if value, err := cc.Get("awesome-key"); err == nil {
        fmt.Printf("Value: %d\n", value)
    }
    
    // Check if key exists
    if cc.Has("awesome-key") {
        fmt.Println("Key exists!")
    }
    
    // Get cache statistics
    fmt.Printf("Cache size: %d\n", cc.Len(true))
}
```

## Core Concepts

### Cache Implementation

The main `Cache` struct implements the `Cacher` interface with additional features:

```go
type Cache[TKey comparable, TValue any] struct {
    // ... internal fields
}
```

## Usage Examples

### Basic Operations

```go
// Create cache with 30 minutes TTL
cache := cache.New[string, User](30 * time.Minute)

// Set values
cache.Set("user:123", User{ID: "123", Name: "John"})
cache.SetWithExpire("temp:data", "temporary", 5*time.Minute)

// Get values
user, err := cache.Get("user:123")
if err != nil {
    // Handle cache miss
}

// Check existence
if cache.Has("user:123") {
    // Key exists and is not expired
}

// Remove specific key
cache.Remove("user:123")

// Get all keys (excluding expired)
keys := cache.Keys(true)

// Get cache size (excluding expired)
size := cache.Len(true)

// Clear all data
cache.Purge()
```

### Lazy Loading

The cache can automatically load missing values using loader functions:

```go
// Simple loader function
loader := func(key string) (User, error) {
    // Fetch user from database
    return fetchUserFromDB(key)
}

// Set loader function
cache.LoaderFunc(loader)

// Now Get will automatically load missing values
user, err := cache.Get("user:456") // Will call loader if not in cache
```

### Advanced Loading with Expiration Control

```go
// Loader function that also controls expiration
loaderWithExpire := func(key string) (User, *time.Duration, error) {
    user, err := fetchUserFromDB(key)
    if err != nil {
        return User{}, nil, err
    }
    
    // Set custom expiration based on user type
    var expiration time.Duration
    if user.IsPremium {
        expiration = 24 * time.Hour
    } else {
        expiration = 1 * time.Hour
    }
    
    return user, &expiration, nil
}

cache.LoaderExpireFunc(loaderWithExpire)
```

### Atomic Updates

Update existing values atomically:

```go
// Update with default TTL
cache.Update("counter", func(current int) int {
    return current + 1
})

// Update with custom TTL
cache.UpdateWithExpire("counter", func(current int) int {
    return current + 1
}, 10*time.Minute)
```

### Expired Items Handling

Expired items are automatically removed from the cache in the following cases:

- on each `Set` call if enough time has passed since the last cleanup - all expired items are removed from the cache
- on each `Get` call if the item is expired
- on every new item loaded from the loader function if enough time has passed since the last cleanup

### Event Hooks

Monitor cache events with callback functions:

```go

// Hook for when items are added to the cache (e.g. to post some metrics)
cache.AddedFunc(func(key string, value User) {
    log.Printf("Added user %s to cache", key)
    metrics.Increment("cache.adds")
})

// Hook for when items are loaded
cache.LoaderFunc(func(key string) (User, error) {
    log.Printf("Loading user %s from database", key)
    return fetchUserFromDB(key)
})
```

## Advanced Features

### Custom Key/Value Types

The cache works with any comparable key type and any value type:

```go
// Custom struct as key
type CacheKey struct {
    UserID   string
    Resource string
}

// Custom struct as value
type CacheValue struct {
    Data      []byte
    Timestamp time.Time
    Metadata  map[string]string
}

// Create cache with custom types
cache := cache.New[CacheKey, CacheValue](1 * time.Hour)

// Use custom types
key := CacheKey{UserID: "123", Resource: "profile"}
value := CacheValue{
    Data:      []byte("user data"),
    Timestamp: time.Now(),
    Metadata:  map[string]string{"version": "1.0"},
}

cache.Set(key, value)
```

### Expiration Strategies

```go
// Never expire
cache.SetWithExpire("permanent", "data", 0)

// Use default TTL
cache.Set("default-ttl", "data")

// Custom TTL
cache.SetWithExpire("short-lived", "data", 30*time.Second)

// Negative TTL falls back to default
cache.SetWithExpire("fallback", "data", -1)
```

## Performance Considerations

- **Memory Usage**: The cache stores all items in memory, so monitor memory consumption
- **Concurrency**: Uses read-write mutexes for optimal concurrent read performance
- **Expiration**: Expired items are automatically cleaned up on access (on one hand it doesn't spin up additional goroutines for expiration, on the other hand it's not the best for memory usage so it will ne reconsidered in future)
- **Loader Functions**: Consider implementing timeouts and error handling in your loader functions

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for a detailed history of changes.

## Dependencies

- Go 1.24+
- [testify](https://github.com/stretchr/testify) (for testing only)
