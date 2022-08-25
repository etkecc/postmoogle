package utils

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// StripHTMLTag from text
//
// Source: https://siongui.github.io/2018/01/16/go-remove-html-inline-style/
func StripHTMLTag(text, tag string) (string, error) {
	doc, err := html.Parse(strings.NewReader(text))
	if err != nil {
		return "", err
	}
	stripHTMLTag(doc, tag)

	var out bytes.Buffer
	err = html.Render(&out, doc)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

func stripHTMLTag(node *html.Node, tag string) {
	i := -1
	for index, attr := range node.Attr {
		if attr.Key == tag {
			i = index
			break
		}
	}
	if i != -1 {
		node.Attr = append(node.Attr[:i], node.Attr[i+1:]...)
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		stripHTMLTag(child, tag)
	}
}
