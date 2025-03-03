# linkpearl

> [more about that name](https://ffxiv.gamerescape.com/wiki/Linkpearl)

A wrapper around [mautrix-go](https://github.com/mautrix/go) with infrastructure/glue code included

## How to get

```
go get github.com/etkecc/go-linkpearl
```

```
lp, err := linkpearl.New(&linkpearl.Config{
	// your options here
})
if err != nil {
	panic(err)
}

go lp.Start()
```

## TODO

* Unit tests

## Features

* Zero configuration End-to-End encryption
* Zero configuration persistent storage
* Zero configuration session restores
* Zero configuration room and user account data encryption with AES GCM (both keys and values)
* Zero configuration room and user account data caching
* [Shared Secret Auth](https://github.com/devture/matrix-synapse-shared-secret-auth) support
* Threads support
* All wrapped components exported
