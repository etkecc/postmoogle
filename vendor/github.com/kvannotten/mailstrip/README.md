# mailstrip

mailstrip is a [Go][2] library that parses email text and strips it of
signatures and reply quotes. It is a port of [email\_reply\_parser][1], GitHub's
library for parsing email replies.

## Differences to email_reply_parser

Most of mailstrip is a line-by-line port of email\_reply\_parser and it passes
all tests from the email\_reply\_parser test suite. However, it also implements
a few improvements that are not part of email\_reply\_parser:

* Forwarded fragments are detected and considered to be visible text, see
  [d321c1][3].
* Replies from Yahoo! which lack ">" quote indicators are handled correctly,
  see [e844d][4].
* Alternative quote headers used by gmail are handled correctly, see
  [7ecb6][5]
* Replies from Google inbox / gmail that has a quoute header in swedish(and possibly other languages) are handled. See [4128d][6].

## Documentation

The API documentation can be found here:
http://godoc.org/github.com/kvannotten/mailstrip

## License

MIT License. See LICENSE file.

[1]: https://github.com/github/email_reply_parser
[2]: http://golang.org/
[3]: https://github.com/kvannotten/mailstrip/commit/d321c10543f77c0beaacb40b04511e619f0652c6
[4]: https://github.com/kvannotten/mailstrip/commit/e844df52342787c3cf2e0ebb8850b16e35f7f437
[5]: https://github.com/kvannotten/mailstrip/commit/7ecb608981016c5633575cb93abb00e4c7370bcf
[6]: https://github.com/kvannotten/mailstrip/commit/4128d1860b0b9477145ac4b4bbf14d1f072f7a4c
