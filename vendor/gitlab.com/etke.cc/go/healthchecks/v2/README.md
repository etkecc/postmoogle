# healthchecks

A [healthchecks.io](https://github.com/healthchecks/healthchecks) client

check the godoc for information

```go
hc := healthchecks.New(
    healthchecks.WithCheckUUID("your-uuid"),
)
go hc.Auto()

hc.Log(strings.NewReader("optional body you can attach to any action"))
hc.Shutdown()
```
