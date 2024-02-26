// mailstrip is a Go library that parses email text and strips it of
// signatures and reply quotes. It is a port of email_reply_parser,
// GitHub's library for parsing email replies.
//
// see https://github.com/github/email_reply_parser
package mailstrip

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
)

// Parse parses a plaintext email and returns the results.
func Parse(text string) Email {
	p := &parser{}
	return p.Parse(text)
}

type parser struct {
	// This determines if any 'visible' Fragment has been found.  Once any
	// visible Fragment is found, stop looking for hidden ones.
	foundVisible bool
	// This instance variable points to the current Fragment.  If the matched
	// line fits, it should be added to this Fragment.  Otherwise, finish it and
	// start a new Fragment.
	fragment *Fragment
	// The fragments parsed so far
	fragments []*Fragment
}

// > I define UNIX as “30 definitions of regular expressions living under one
// > roof.”
// —Don Knuth
//
// Porting the Ruby regular expressions from email_reply_parser to Go required
// making the following changes:
//
// - Unlike most regexp flavors I'm familiar with, ^ and $ stand for beginning
//   and end of line respectively in Ruby. Getting the same behavior in Go
//   required enabling Go's multiline mode "(?m)" for these expressions.
// - Ruby's multiline mode "/m" is the same as Go's "(?s)" flag. Both are used
//   to make "." match "\n" characters.
var (
	// used to join quote headers that were broken into multiple lines by the
	// e-mail client. e.g. gmail does that for lines exceeding 80 chars
	multiLineReplyHeaderRegexps = []*regexp.Regexp{
		// e.g. On Aug 22, 2011, at 7:37 PM, defunkt<reply@reply.github.com> wrote:
		regexp.MustCompile("(?sm)^(On\\s(?:.+)wrote:)$"),
		// e.g. 2013/11/13 John Smith <john@smith.org>
		regexp.MustCompile("(?sm)^(\\d{4}/\\d{1,2}/\\d{1,2} .*<.+@.+>)$"),
	}
	sigRegexp         = regexp.MustCompile("(\\d+ swodniW rof >.*<liaM morf tneS|--|__|(?m)\\w-$)|(?m)(^(\\w+\\s*){1,3} " + reverseString("Sent from my") + "$)")
	fwdRegexp         = regexp.MustCompile("(?mi)^--+\\s*" + reverseString("Forwarded message") + "\\s*--+$")
	quotedRegexp      = regexp.MustCompile("(?m)(>+)$")
	quoteHeaderRegexp = regexp.MustCompile("(?m)^:etorw.*nO$|^.*[0-9]{4}\\s\\.\\w{2,4}\\s\\d{1,2}\\s.{3,4}$|^\\w{3,4}\\s\\d{1,2}\\s\\w{3,4}\\.\\s[0-9]{4}.*$|^>.*\\d{1,2}/\\d{1,2}/\\d{4}$|^(?m)^.*?[0-9]{4}\\s\\.\\w+\\s\\d\\s.*n\\.*$")
)

func (p *parser) Parse(text string) Email {
	// Normalize line endings.
	text = strings.Replace(text, "\r\n", "\n", -1)

	// Check for multi-line reply headers. Some clients break up the "On DATE,
	// NAME <EMAIL> wrote:" line (and similar quote headers) into multiple lines.
	for _, r := range multiLineReplyHeaderRegexps {
		if m := r.FindStringSubmatch(text); len(m) == 2 {
			// Remove all new lines from the reply header.
			text = strings.Replace(text, m[1], strings.Replace(m[1], "\n", "", -1), -1)
		}
	}

	// The text is reversed initially due to the way we check for hidden
	// fragments.
	text = reverseString(text)

	// Use the Reader to pull out each line of the email content.
	reader := bufio.NewReader(strings.NewReader(text))
	for {
		line, e := reader.ReadBytes('\n')
		p.scanLine(strings.TrimRight(string(line), "\n"))
		if e == io.EOF {
			break
		} else if e != nil {
			// Our underlaying reader is a strings.Reader, which will never return
			// errors other than io.EOF, so this is merely a sanity check.
			panic(fmt.Sprintf("Bug: ReadBytes returned an error other than io.EOF: %#v", e))
		}
	}

	// Finish up the final fragment.  Finishing a fragment will detect any
	// attributes (hidden, signature, reply), and join each line into a
	// string.
	p.finishFragment()

	// Now that parsing is done, reverse the order.
	reverseFragments(p.fragments)
	return Email(p.fragments)
}

// scaneLine scans the given line of text and figures out which fragment it
// belongs to.
func (p *parser) scanLine(line string) {
	sigMatch := sigRegexp.MatchString(line)

	if !sigMatch {
		line = strings.TrimLeftFunc(line, unicode.IsSpace)
	}

	// We're looking for leading `>`'s to see if this line is part of a
	// quoted Fragment.
	isQuoted := quotedRegexp.MatchString(line)

	// Mark the current Fragment as a signature if the current line is empty
	// and the Fragment starts with a common signature indicator.
	if p.fragment != nil && line == "" {
		// lastLine is really the first line, since the lines are still reversed
		// at this point.
		lastLine := p.fragment.lines[len(p.fragment.lines)-1]
		if fwdRegexp.MatchString(lastLine) {
			p.fragment.forwarded = true
			p.finishFragment()
		} else if sigRegexp.MatchString(lastLine) {
			p.fragment.signature = true
			p.finishFragment()
		}
	}

	isQuoteHeader := p.quoteHeader(line)
	// Yahoo! does not use '>' quote indicator in replies, so if a quote header
	// suddenly appears in an otherwise unquoted fragment, consider it quoted
	// now.
	if p.fragment != nil && isQuoteHeader {
		p.fragment.quoted = true
	}

	// If the line matches the current fragment, add it.  Note that a common
	// reply header also counts as part of the quoted Fragment, even though
	// it doesn't start with `>`.
	if p.fragment != nil &&
		((p.fragment.quoted == isQuoted) ||
			(p.fragment.quoted && (isQuoteHeader || line == ""))) {
		p.fragment.lines = append(p.fragment.lines, line)

		// Otherwise, finish the fragment and start a new one.
	} else {
		p.finishFragment()
		p.fragment = &Fragment{quoted: isQuoted, lines: []string{line}}
	}
}

// quoteHeader detects if a given line is a header above a quoted area.  It is
// only checked for lines preceding quoted regions. Returns true if the line is
// a valid header, or false.
func (p *parser) quoteHeader(line string) bool {
	return quoteHeaderRegexp.MatchString(line)
}

// finishFragment builds the fragment string and reverses it, after all lines
// have been added.  It also checks to see if this Fragment is hidden.  The
// hidden Fragment check reads from the bottom to the top.
//
// Any quoted Fragments or signature Fragments are marked hidden if they are
// below any visible Fragments.  Visible Fragments are expected to contain
// original content by the author.  If they are below a quoted Fragment, then
// the Fragment should be visible to give context to the reply.
//
//     some original text (visible)
//
//     > do you have any two's? (quoted, visible)
//
//     Go fish! (visible)
//
//     > -- > Player 1 (quoted, hidden)
//
//     -- Player 2 (signature, hidden)
func (p *parser) finishFragment() {
	if p.fragment != nil {
		p.fragment.finish()
		if !p.foundVisible {
			if p.fragment.quoted || p.fragment.signature ||
				strings.TrimSpace(p.fragment.String()) == "" {
				p.fragment.hidden = true
			} else {
				p.foundVisible = true
			}
		}
		p.fragments = append(p.fragments, p.fragment)
	}
	p.fragment = nil
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func reverseFragments(f []*Fragment) {
	for i, j := 0, len(f)-1; i < j; i, j = i+1, j-1 {
		f[i], f[j] = f[j], f[i]
	}
}

// Email contains the parsed contents of an email.
type Email []*Fragment

// String returns the non-Hidden() fragments of the Email.
func (e Email) String() string {
	results := []string{}
	for _, fragment := range e {
		if fragment.Hidden() {
			continue
		}

		results = append(results, fragment.String())
	}

	result := strings.Join(results, "\n")
	result = strings.TrimRightFunc(result, unicode.IsSpace)
	return result
}

// Fragment contains a parsed section of an email.
type Fragment struct {
	lines     []string
	content   string
	hidden    bool
	signature bool
	forwarded bool
	quoted    bool
}

// finish builds the string content by joining the lines and reversing them.
func (f *Fragment) finish() {
	f.content = strings.Join(f.lines, "\n")
	f.lines = nil
	f.content = reverseString(f.content)
}

// Forwarded returns if the fragment is forwarded or not.
func (f *Fragment) Forwarded() bool {
	return f.forwarded
}

// Signature returns if the fragment is a signature or not.
func (f *Fragment) Signature() bool {
	return f.signature
}

// Signature returns if the fragment is a quote or not.
func (f *Fragment) Quoted() bool {
	return f.quoted
}

// Signature returns if the fragment is considered hidden or not.
func (f *Fragment) Hidden() bool {
	return f.hidden
}

// String returns the content of the fragment.
func (f *Fragment) String() string {
	return f.content
}
