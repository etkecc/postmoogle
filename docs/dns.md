# DNS configuration

the following configuration is required only if you want to send emails from Postmoogle

# MX

Add a new MX DNS record of the `MX` type for your domain that will be used with postmoogle.
It should point to the same (sub-)domain.
Looks odd, but some mail servers will refuse to interact with your mail server 
(and Postmoogle is already a mail server) without MX records.

<details>
<summary>Example</summary>

```bash
dig MX example.com

; <<>> DiG 9.18.6 <<>> MX example.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12688
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232
;; QUESTION SECTION:
;example.com.			IN	MX

;; ANSWER SECTION:
example.com.		1799	IN	MX	10 example.com.

;; Query time: 40 msec
;; SERVER: 1.1.1.1#53(1.1.1.1) (UDP)
;; WHEN: Tue Sep 06 16:44:47 EEST 2022
;; MSG SIZE  rcvd: 59
```

</details>

# SPF

Aadd a new SPF DNS record of the `TXT` type for your domain that will be used with Postmoogle, 
with format: `v=spf1 ip4:SERVER_IP4 -all` (replace `SERVER_IP4` with your server's IP address),
for servers with IPv6: `v=spf1 ip6:SERVER_IP6 -all` (you may use both `ip4` and `ip6` in one TXT record).

<details>
<summary>Example</summary>

```bash
$ dig txt example.com

; <<>> DiG 9.18.6 <<>> txt example.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 24796
;; flags: qr rd ra; QUERY: 1, ANSWER: 4, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232
;; QUESTION SECTION:
;example.com.			IN	TXT

;; ANSWER SECTION:
example.com.		1799	IN	TXT	"v=spf1 ip4:111.111.111.111 -all"

;; Query time: 36 msec
;; SERVER: 1.1.1.1#53(1.1.1.1) (UDP)
;; WHEN: Sun Sep 04 21:35:04 EEST 2022
;; MSG SIZE  rcvd: 255
```

</details>

# DMARC

Add a new DMARC DNS record of the `TXT` type for subdomain `_dmarc` with a proper policy.
The simplest policy you can use is: `v=DMARC1; p=quarantine;`.

<details>
<summary>Example</summary>

```bash
$ dig txt _dmarc.example.com

; <<>> DiG 9.18.6 <<>> txt _dmarc.example.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 57306
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232
;; QUESTION SECTION:
;_dmarc.example.com.			IN	TXT

;; ANSWER SECTION:
_dmarc.example.com.		1799	IN	TXT	"v=DMARC1; p=quarantine;"

;; Query time: 46 msec
;; SERVER: 1.1.1.1#53(1.1.1.1) (UDP)
;; WHEN: Sun Sep 04 21:31:30 EEST 2022
;; MSG SIZE  rcvd: 79
```

</details>

# DKIM

Add new DKIM DNS record of `TXT` type for subdomain `postmoogle._domainkey` that will be used with postmoogle.
You can get that signature using the `!pm dkim` command:

<details>
<summary>!pm dkim</summary>

DKIM signature is: `v=DKIM1; k=rsa; p=MIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEAxJVqmBHhK9FY93q1o3WEaP2GKMh/3LNMyvi1uSjOKxIyfWv685KxX1EUrbHakQRCTtUM7efKEsXsXBh+DQru2TE32yFpL9afA5BbHj3KePGFY8KJ2m0sQxbQcvn2KjJC0IQ15mk0rninPhtphU/2zLsd6e7Rl1m3L+9Osk320GbfDgSKjRPcSiwVMbLJpSOP0H0F3cIu+c1fHZHfmWy0O+us42C3HTLTlD779LTnQnKlAOQD/+DYYqz6TGGxEwUG2BRQ8O8w7/wXEkg/6a/MxNtPnc59g29CpqRsDkuYiR3UIpqzLDoqHlaoKNbYy34R+4aIjfNpmZyR5kIumws+3MJtJt9UhBTMloqd8lZDPaPmX2NEDqbcSTkHMQrphk+EWSCc7OvbKRaXZ0SyJLpLjxRwKrpeO0JAI0ZpnAFS11uBEe9GSS8uzIIFNYVD1vHloAFKvUJEhyuVyz9/SyqTnArN3ZTiC5cqD1MB86q5QPrKqZfp1dAnv7xAJThL0AP/AgMBAAE=`.
You need to add it to your DNS records (if not already):
Add new DNS record with type = `TXT`, key (subdomain/from): `postmoogle._domainkey` and value (to):

```
v=DKIM1; k=rsa; p=MIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEAxJVqmBHhK9FY93q1o3WEaP2GKMh/3LNMyvi1uSjOKxIyfWv685KxX1EUrbHakQRCTtUM7efKEsXsXBh+DQru2TE32yFpL9afA5BbHj3KePGFY8KJ2m0sQxbQcvn2KjJC0IQ15mk0rninPhtphU/2zLsd6e7Rl1m3L+9Osk320GbfDgSKjRPcSiwVMbLJpSOP0H0F3cIu+c1fHZHfmWy0O+us42C3HTLTlD779LTnQnKlAOQD/+DYYqz6TGGxEwUG2BRQ8O8w7/wXEkg/6a/MxNtPnc59g29CpqRsDkuYiR3UIpqzLDoqHlaoKNbYy34R+4aIjfNpmZyR5kIumws+3MJtJt9UhBTMloqd8lZDPaPmX2NEDqbcSTkHMQrphk+EWSCc7OvbKRaXZ0SyJLpLjxRwKrpeO0JAI0ZpnAFS11uBEe9GSS8uzIIFNYVD1vHloAFKvUJEhyuVyz9/SyqTnArN3ZTiC5cqD1MB86q5QPrKqZfp1dAnv7xAJThL0AP/AgMBAAE=
```

Without that record other email servers may reject your emails as spam, kupo.

</details>

<details>
<summary>Example</summary>

```bash
$ dig TXT postmoogle._domainkey.example.com

; <<>> DiG 9.18.6 <<>> TXT postmoogle._domainkey.example.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 59014
;; flags: qr rd ra; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 1232
;; QUESTION SECTION:
;postmoogle._domainkey.example.com.	IN	TXT

;; ANSWER SECTION:
postmoogle._domainkey.example.com. 600	IN TXT  "v=DKIM1; k=rsa; p=MIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEAxJVqmBHhK9FY93q1o3WEaP2GKMh/3LNMyvi1uSjOKxIyfWv685KxX1EUrbHakQRCTtUM7efKEsXsXBh+DQru2TE32yFpL9afA5BbHj3KePGFY8KJ2m0sQxbQcvn2KjJC0IQ15mk0rninPhtphU/2zLsd6e7Rl1m3L+9Osk320GbfDgSKjRPcSiwVMbLJpSOP0H0F3cIu+c1fHZHfmWy0O+us42C3HTLTlD779LTnQnKlAOQD/+DYYqz6TGGxEwUG2BRQ8O8w7/wXEkg/6a/MxNtPnc59g29CpqRsDkuYiR3UIpqzLDoqHlaoKNbYy34R+4aIjfNpmZyR5kIumws+3MJtJt9UhBTMloqd8lZDPaPmX2NEDqbcSTkHMQrphk+EWSCc7OvbKRaXZ0SyJLpLjxRwKrpeO0JAI0ZpnAFS11uBEe9GSS8uzIIFNYVD1vHloAFKvUJEhyuVyz9/SyqTnArN3ZTiC5cqD1MB86q5QPrKqZfp1dAnv7xAJThL0AP/AgMBAAE="

;; Query time: 90 msec
;; SERVER: 1.1.1.1#53(1.1.1.1) (UDP)
;; WHEN: Mon Sep 05 16:16:21 EEST 2022
;; MSG SIZE  rcvd: 525
```

</details>

# rDNS

> additional PTR record will help you to get better spam score

Configure Reverse DNS of your server. Unfortunately, rDNS is provider-specific, so you have to find out how to configure it with your hosting provider. Search for something like: `PROVIDER configure "rdns"` (where `PROVIDER` is your hosting provider name)
