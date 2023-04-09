# fswatcher

A handy wrapper around [fsnotify](https://github.com/fsnotify/fsnotify) with deduplication

```go
watcher := fswatcher.New([]string{"/tmp/your-file.txt"}, 0)
defer watcher.Stop()
go watcher.Start(func(e fsnotify.Event){
	// do something with event
})
```
