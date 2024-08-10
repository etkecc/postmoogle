# tricks

<!-- vim-markdown-toc GitLab -->

* [Logs](#logs)
    * [get most active hosts](#get-most-active-hosts)

<!-- vim-markdown-toc -->

## Logs

### get most active hosts

Even if you use postmoogle as an internal mail server and contact "outside internet" quite rarely,
you will see lots of connections to your SMTP servers from random hosts over internet that do... nothing?
They don't send any valid emails or do something meaningful, thus you can safely assume they are spammers.

To get top X (in example: top 10) hosts with biggest count of attempts to connect to your postmoogle instance, run the following one-liner:

```bash
journalctl -o cat -u postmoogle | grep "accepted connection" | grep -oE "[0-9]{1,3}.[0-9]{1,3}.[0-9]{1,3}.[0-9]{1,3}:" | sort | uniq -ci | sort -rn | head -n 10
    253 111.111.111.111
    183 222.222.222.222
     39 333.333.333.333
     38 444.444.444.444
     18 555.555.555.555
     16 666.666.666.666
      8 777.777.777.777
      5 888.888.888.888
      5 999.999.999.999
      4 010.010.010.010
```

of course, IP addresses above are crafted just to visualize their place in that top, according to the number of connections done.
In reality, you will see real IP addresses here. Usually, only hosts with hundreds or thousands of connections for the last 7 days worth checking.

What's next?
Do **not** ban them right away. Check WHOIS info for each host and only after that decide if you really want to ban that host or not.
