# trySMTP

Library that tries to connect to SMTP host by TO/target email address:

* Lookup MX and A
* Try to connect to SMTP on different ports
* Try to use STARTTLS if server supports it
* Return SMTP Client with `Mail()` and `Rcpt()` already called


```go
from := "sender@example.com"
to := "target@example.org"
client, err := trysmtp.Connect(from, to)
if err != nil {
	// something went wrong!
}

client.Data([]byte("your email data here"))
```
