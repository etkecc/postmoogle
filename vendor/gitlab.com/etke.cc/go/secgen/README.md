# secgen

Secrets generator, supported types:

* Passwords
* ED25519 SSH keypairs

Simple go environment variables reader, for env-based configs

```go
// password
securePassword := secgen.Password(64)

// ssh key
publicSSHkey, privateSSHkey, err := secgen.Keypair()
```
