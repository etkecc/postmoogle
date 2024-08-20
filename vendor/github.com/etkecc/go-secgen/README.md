# secgen

Secrets generator, supported types:

* Passwords
* Random bytes
* ED25519 SSH keypairs

Simple go environment variables reader, for env-based configs

```go
// password
securePassword := secgen.Password(64)
// base64-encode random bytes (openssl rand -base64 replacement)
randomBytes := secgen.Base64Bytes(64)

// ssh key
publicSSHkey, privateSSHkey, err := secgen.Keypair()
```
