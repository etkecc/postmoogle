package email

import (
	"os"
	"strings"
	"testing"
	"time"
)

func testResolver(src string) *Image {
	if strings.Contains(src, "missing") {
		return nil
	}
	return &Image{URI: "mxc://example.com/" + src[strings.LastIndex(src, "/")+1:], Width: 40, Height: 20}
}

func TestRenderHTML(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"paragraphs": {
			input:    `<p>first</p><p>second</p>`,
			expected: `<p>first</p><p>second</p>`,
		},
		"divs are lines, empty divs are blank lines": {
			input:    `<div>a</div><div>b</div><div><br></div><div>c</div>`,
			expected: `<p>a<br>b</p><p>c</p>`,
		},
		"non-breaking space blocks are blank lines": {
			input:    `<div>a</div><div>&nbsp;</div><div>b&nbsp;<b>&nbsp;</b>c</div>`,
			expected: `<p>a</p><p>b c</p>`,
		},
		"outlook paragraphs": {
			input:    `<p class="MsoNormal">a<o:p></o:p></p><p class="MsoNormal"><o:p>&nbsp;</o:p></p><p class="MsoNormal">b</p><p class="MsoNormal">c</p>`,
			expected: `<p>a</p><p>b<br>c</p>`,
		},
		"layout table cells are paragraphs": {
			input:    `<table><tr><td>Sign in to Example</td></tr><tr><td>Click the button below.</td></tr></table>`,
			expected: `<p>Sign in to Example</p><p>Click the button below.</p>`,
		},
		"short cells share a line": {
			input:    `<table><tr><td>Name:</td><td>Value</td></tr><tr><td>Help</td><td></td><td>Privacy</td></tr></table>`,
			expected: `<p>Name: Value<br>Help · Privacy</p>`,
		},
		"cells with blocks are stacked": {
			input:    `<table><tr><td><p>left</p></td><td><p>right</p></td></tr></table>`,
			expected: `<p>left</p><p>right</p>`,
		},
		"data tables are kept": {
			input:    `<table><tr><th>Item</th><th>Qty</th></tr><tr><td>Apple</td><td>2</td></tr></table>`,
			expected: `<table><tr><th>Item</th><th>Qty</th></tr><tr><td>Apple</td><td>2</td></tr></table>`,
		},
		"hidden content is dropped": {
			input:    `<div style="display:none">preview</div><div style="max-height:0;overflow:hidden">mobile</div><span hidden>x</span><p style="opacity: 0">y</p><p>text</p>`,
			expected: `<p>text</p>`,
		},
		"head, styles, scripts, and forms are dropped": {
			input:    `<html><head><title>Title</title><style>p{color:red}</style></head><body><script>alert(1)</script><p>ok</p><input value="v"><iframe src="https://example.com"></iframe></body></html>`,
			expected: `<p>ok</p>`,
		},
		"noscript is shown like in email clients": {
			input:    `<noscript><p>visible</p></noscript>`,
			expected: `<p>visible</p>`,
		},
		"text is escaped": {
			input:    `<p>&lt;b&gt;not bold&lt;/b&gt; &amp; "quoted"</p>`,
			expected: `<p>&lt;b&gt;not bold&lt;/b&gt; &amp; &#34;quoted&#34;</p>`,
		},
		"unsafe links are removed": {
			input:    `<p><a href="javascript:alert(1)">click</a> <a href="/relative">here</a> <a href="https://example.com/?a=1&amp;b=&quot;2&quot;">safe</a></p>`,
			expected: `<p>click here <a href="https://example.com/?a=1&amp;b=&#34;2&#34;">safe</a></p>`,
		},
		"links without text are removed": {
			input:    `<p>before <a href="https://example.com"><img src="https://example.com/missing.png"></a> after</p>`,
			expected: `<p>before after</p>`,
		},
		"links spanning blocks": {
			input:    `<a href="https://example.com"><div>a</div><div>b</div></a>`,
			expected: `<p><a href="https://example.com">a<br>b</a></p>`,
		},
		"buttons are bold": {
			input:    `<a href="https://example.com/1" style="background-color:#000;padding:10px">One</a><table><tr><td bgcolor="#d97757"><a href="https://example.com/2" style="color:#fff">Two</a></td></tr></table>`,
			expected: `<p><a href="https://example.com/1"><strong>One</strong></a></p><p><a href="https://example.com/2"><strong>Two</strong></a></p>`,
		},
		"css formatting": {
			input:    `<p><span style="font-weight:bold">b</span> <span style="font-style: italic !important">i</span> <span style="text-decoration:line-through">s</span></p>`,
			expected: `<p><strong>b</strong> <em>i</em> <del>s</del></p>`,
		},
		"nested formatting is not duplicated": {
			input:    `<p><b><strong style="font-weight:700">bold</strong></b></p>`,
			expected: `<p><strong>bold</strong></p>`,
		},
		"headings": {
			input:    `<h1>Title</h1><h2>Section</h2><h5>Small</h5>`,
			expected: `<h3>Title</h3><h4>Section</h4><h4>Small</h4>`,
		},
		"long headings are bold paragraphs": {
			input:    `<h1>` + strings.Repeat("word ", 40) + `</h1>`,
			expected: `<p><strong>` + strings.TrimSpace(strings.Repeat("word ", 40)) + `</strong></p>`,
		},
		"lists": {
			input:    `<ul><li>a</li><li>b <b>c</b></li><li style="display:none">hidden</li></ul><ol start="3"><li><p>x</p></li></ol>`,
			expected: `<ul><li>a</li><li>b <strong>c</strong></li></ul><ol start="3"><li>x</li></ol>`,
		},
		"lists without bullets are lines": {
			input:    `<ul style="list-style:none"><li>Help</li><li>Privacy</li></ul>`,
			expected: `<p>Help<br>Privacy</p>`,
		},
		"quotes": {
			input:    `<p>reply</p><blockquote type="cite"><p>quoted</p><p>text</p></blockquote>`,
			expected: `<p>reply</p><blockquote><p>quoted</p><p>text</p></blockquote>`,
		},
		"code blocks": {
			input:    "<pre><code>a &lt; b\n  c</code></pre>",
			expected: "<pre><code>a &lt; b\n  c</code></pre>",
		},
		"preformatted text keeps lines": {
			input:    "<pre>line 1\nline 2 <a href=\"https://example.com\">link</a>\n\nline 4</pre>",
			expected: `<p>line 1<br>line 2 <a href="https://example.com">link</a></p><p>line 4</p>`,
		},
		"top borders are rules": {
			input:    `<p>Regards</p><div style="border:none;border-top:solid #E1E1E1 1.0pt"><p>From: John</p></div><div style="border-top:0 solid">x</div>`,
			expected: `<p>Regards</p><hr><p>From: John</p><p>x</p>`,
		},
		"rules are not repeated": {
			input:    `<hr><p>a</p><hr><hr><p>b</p><hr>`,
			expected: `<p>a</p><hr><p>b</p>`,
		},
		"invisible padding characters are removed": {
			input:    `<p>Hello&#847;&zwnj; &zwnj;&#8203;&shy;world&#65279;</p>`,
			expected: `<p>Hello world</p>`,
		},
		"joiners inside words and emoji are kept": {
			input:    "<p>\U0001F468\u200d\U0001F469\u200d\U0001F467 \u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645</p>",
			expected: "<p>\U0001F468\u200d\U0001F469\u200d\U0001F467 \u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645</p>",
		},
		"images": {
			input:    `<p><img src="https://example.com/logo.png" alt="Logo" width="120"> <img src="https://example.com/wide.png" width="1200" height="300"></p>`,
			expected: `<p><img src="mxc://example.com/logo.png" alt="Logo" width="120" height="60"> <img src="mxc://example.com/wide.png" width="600" height="150"></p>`,
		},
		"missing images show their alt text": {
			input:    `<p><img src="https://example.com/missing.png" alt="Company logo"> <a href="https://example.com/x"><img src="https://example.com/missing-x.png" alt="X"></a></p>`,
			expected: `<p>Company logo <a href="https://example.com/x">X</a></p>`,
		},
		"tracking pixels and spacers are dropped": {
			input:    `<p>text<img src="https://example.com/open.gif" width="1" height="1"><img src="https://example.com/spacer.gif" style="width:20px;height:0"><img src="https://example.com/hidden.png" style="display:none"></p>`,
			expected: `<p>text</p>`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			output := strings.Join(renderHTML(test.input, testResolver, false), "")
			if output != test.expected {
				t.Errorf("\nexpected: %s\n  output: %s", test.expected, output)
			}
		})
	}
}

func TestRenderHTML_TinyNaturalSize(t *testing.T) {
	resolve := func(string) *Image { return &Image{URI: "mxc://example.com/pixel", Width: 1, Height: 1} }

	output := strings.Join(renderHTML(`<p>text <img src="https://example.com/open.gif"></p>`, resolve, false), "")

	if output != "<p>text</p>" {
		t.Errorf("tracking pixel was not removed: %s", output)
	}
}

func TestRenderHTML_FlatTables(t *testing.T) {
	input := `<table><tr><th>Item</th><th>Qty</th></tr><tr><td>Apple</td><td>2</td></tr></table>`

	output := strings.Join(renderHTML(input, nil, true), "")

	if output != "<p>Item · Qty<br>Apple · 2</p>" {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestRenderHTML_DeepNesting(t *testing.T) {
	input := strings.Repeat("<div><table><tr><td>", 90) + "deep" + strings.Repeat("</td></tr></table></div>", 90) + "<p>after</p>"
	started := time.Now()

	output := strings.Join(renderHTML(input, nil, false), "")

	if output != "<p>deep</p><p>after</p>" {
		t.Errorf("unexpected output: %s", output)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("rendering took too long: %s", elapsed)
	}
}

func TestRenderHTML_TooDeep(t *testing.T) {
	input := strings.Repeat("<div>", 1000) + "deep" + strings.Repeat("</div>", 1000)

	output := renderHTML(input, nil, false)

	if len(output) != 0 {
		t.Errorf("HTML rejected by the parser should not be rendered, got: %v", output)
	}
}

func TestRenderHTML_Transactional(t *testing.T) {
	input, err := os.ReadFile("testdata/transactional.html")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{
		`<p><img src="mxc://example.com/logo.png" alt="Example" width="120" height="28"></p>`,
		`<p><strong>Sign in to Example</strong></p>`,
		`<p>Click the button below to finish signing in. This link expires in 10 minutes.</p>`,
		`<p><a href="https://example.com/magic-link#token"><strong>Sign in</strong></a></p>`,
		`<p>If you didn&#39;t request this email, you can safely ignore it.</p>`,
		`<p>Need a hand? Contact <a href="https://track.example.com/ls/click?upn=abc">Example Support</a>.</p>`,
		`<p>Example, Inc<br>548 Market St, PMB 90375<br>San Francisco, CA 94104</p>`,
		`<p><a href="https://track.example.com/help">Help</a> · <a href="https://track.example.com/privacy">Privacy</a></p>`,
		`<p><a href="https://track.example.com/x"><img src="mxc://example.com/x.png" width="20" height="20"></a> ` +
			`<a href="https://track.example.com/linkedin"><img src="mxc://example.com/linkedin.png" width="20" height="20"></a> ` +
			`<a href="https://track.example.com/youtube"><img src="mxc://example.com/youtube.png" alt="YouTube" width="20" height="20"></a></p>`,
	}

	output := renderHTML(string(input), testResolver, false)

	if strings.Join(output, "\n") != strings.Join(expected, "\n") {
		t.Errorf("\nexpected:\n%s\n\noutput:\n%s", strings.Join(expected, "\n"), strings.Join(output, "\n"))
	}
}

func TestDisplaySize(t *testing.T) {
	tests := map[string]struct {
		width, height, naturalWidth, naturalHeight int
		expectedWidth, expectedHeight              int
	}{
		"attributes win":         {100, 50, 400, 400, 100, 50},
		"width keeps the ratio":  {100, -1, 400, 200, 100, 50},
		"height keeps the ratio": {-1, 50, 400, 200, 100, 50},
		"natural size":           {-1, -1, 300, 200, 300, 200},
		"wide images shrink":     {-1, -1, 1200, 400, 600, 200},
		"unknown natural size":   {120, -1, 0, 0, 120, -1},
		"huge natural size":      {-1, -1, 1 << 30, 1 << 30, 600, 600},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			width, height := displaySize(test.width, test.height, test.naturalWidth, test.naturalHeight)
			if width != test.expectedWidth || height != test.expectedHeight {
				t.Errorf("expected %dx%d, got %dx%d", test.expectedWidth, test.expectedHeight, width, height)
			}
		})
	}
}
