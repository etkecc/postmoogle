# crypter

[![Go Reference](https://pkg.go.dev/badge/github.com/etkecc/go-kit.svg)](https://pkg.go.dev/github.com/etkecc/go-kit/crypter)

Transparent AES-GCM encryption for string values. Feed it a plaintext, get back `ENCv1[...]`; feed it that back, get your plaintext. The point is that you can run it over a config map that mixes encrypted and plaintext fields without tracking which is which, it does the right thing to each.

Part of the dependency-free root module. This one is the crypto primitive; if you want to encrypt values *inside a YAML file in place*, that's the [`crypter/yaml`](./yaml) sibling.

```go
go get github.com/etkecc/go-kit
```

```go
import "github.com/etkecc/go-kit/crypter"

c, err := crypter.New(os.Getenv("SECRET_KEY")) // SECRET_KEY=$(pwgen -s 32 1)
if err != nil {
    log.Fatal(err)
}

sealed, _ := c.Encrypt("hunter2")   // "ENCv1[...]"
plain, _  := c.Decrypt(sealed)      // "hunter2"

c.Encrypt(sealed) // already tagged, passes through unchanged
c.Decrypt("plain-value") // not tagged, passes through unchanged
```

## The key is not a password

The secret handed to `New` must be **exactly** 16, 24, or 32 bytes of raw key material (AES-128 / 192 / 256). It's used directly as the AES key with zero stretching, no KDF, no salt. So do not hand it a memorable passphrase, that's a 32-character wish, not a key. Use `pwgen -s 32 1` or `crypto/rand`. Wrong length is `ErrInvalidKeyLength` and you'll know immediately.

## What you get, and the one sharp edge

- **Format:** `ENCv1[<base64url-raw(nonce || ciphertext || tag)>]`. 12-byte random nonce per call, so encrypting the same value twice gives two different ciphertexts. GCM's tag means a tampered value fails to decrypt (`ErrOpen`) instead of returning garbage.
- **Thread-safe:** one `Crypter` serves any number of goroutines. The nonce pool is a `sync.Pool`, no lock in the hot path.
- **Idempotent both ways:** the mixed-config behavior above is the whole reason it exists.
- **The catch:** a plaintext value must never itself begin with `ENCv1[`. Nothing enforces this, and if you violate it, `Encrypt` thinks it's already sealed and stores your plaintext in the clear. Don't prefix your secrets with the start tag. In practice nobody does, but now you know why not to.

Full error list in [godoc](https://pkg.go.dev/github.com/etkecc/go-kit/crypter).

## License

GNU LGPL-3.0. See [../LICENSE](../LICENSE).
