# Healthchecks.io Golang Client Library

This library provides a Golang client for interacting with [healthchecks.io](https://github.com/healthchecks/healthchecks), a service for monitoring the uptime of your cron jobs, scripts, and background tasks. With this library, you can easily integrate [healthchecks.io](https://github.com/healthchecks/healthchecks) monitoring into your Golang applications.

## Installation

To use this library, simply import it into your Golang project:

```bash
go get github.com/etkecc/go-healthchecks/v2
```

## Usage

### Initialization

To start using the healthchecks.io client, you can initialize a new client with optional configuration using the `New` function:

```go
import "github.com/etkecc/go-healthchecks/v2"

func main() {
    // Initialize a new client with default options
    client := healthchecks.New(healthchecks.WithCheckUUID("your-check-uuid"))
    defer client.Shutdown()

    // Or initialize with custom options
    client := healthchecks.New(
        healthchecks.WithBaseURL("https://hc-ping.com"),
        healthchecks.WithUserAgent("Your Custom User Agent"),
        healthchecks.WithCheckUUID("your-check-uuid"),
        // Add more options as needed
    )
    defer client.Shutdown()
}
```

### Sending Signals

Once initialized, you can send various signals to healthchecks.io to indicate the status of your jobs:

- **Start**: Indicates that the job has started.
- **Success**: Indicates that the job is still running and healthy.
- **Fail**: Indicates that the job has failed.
- **Log**: Adds an event to the job log without changing the job status.
- **ExitStatus**: Sends the job's exit code (0-255).

```go
// Example sending a start signal
client.Start()

// Example sending a success signal
client.Success()

// Example sending a fail signal with a custom message
client.Fail(strings.NewReader("Job failed due to XYZ"))

// Example sending a log signal
client.Log(strings.NewReader("Custom log message"))

// Example sending an exit status signal
client.ExitStatus(1) // Example exit code
```

### Automatic Ping

You can also configure the client to automatically send success signals at regular intervals using the `Auto` function:

```go
import "time"

func main() {
    // Initialize the client
    client := healthchecks.New(healthchecks.WithCheckUUID("your-check-uuid"))
    defer client.Shutdown()

    // Start automatic pinging every 5 seconds
    go client.Auto(5 * time.Second)

    // Your application logic here
}
```

### Shutting Down

When you're done with the client, make sure to shut it down properly to release resources:

```go
client.Shutdown()
```

## Global Client

You can also use a global instance of the client for convenience:

```go
// Use the global client - a previously initialized client `WithGlobal()`
healthchecks.Global()
```

Then you can use `healthchecks.Global()` to access the global client instance throughout your application.

## Configuration Options

The client provides various configuration options via functional options:

- `WithHTTPClient`: Sets the HTTP client used by the client.
- `WithBaseURL`: Sets the base URL for healthchecks.io.
- `WithUserAgent`: Sets the user agent string for HTTP requests.
- `WithErrLog`: Sets a custom error logging function.
- `WithCheckUUID`: Sets the UUID for the health check.
- `WithAutoProvision`: Enables auto-provisioning of the health check.
- `WithGlobal`: Sets this client as the global client.
- `WithDone`: Sets the done channel for graceful shutdown.

Example usage:

```go
client := healthchecks.New(
    healthchecks.WithBaseURL("https://custom-healthchecks-url.com"),
    healthchecks.WithUserAgent("Custom User Agent"),
    // Add more options as needed
)
```
