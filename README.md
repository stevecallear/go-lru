# LRU

LRU is an LRU memory cache for Go.

## Getting Started
`go get github.com/stevecallear/go-lru`

``` go
package main

import (
    "fmt"
    "log"

    lru "github.com/stevecallear/go-lru"
)

func main() {
    c := lru.NewCache(lru.Options{
        Capacity: 1000,
        Policy:   lru.NewFixedExpirationPolicy(),
    })

    r := lru.GetOrAdd{
        Key: "key",
        TTL: 1 * time.Minute,
        Create: func() interface{} {
            return "value"
        },
    }

    if err := c.GetOrAdd(&r); err != nil {
        log.Fatalln(err)
    }

    fmt.Println(r.Result)
}
```