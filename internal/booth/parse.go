package booth

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

type Product struct {
	BoothID     string
	Name        string
	Description string
	CoverURL    string
}

func ParseHTML(body []byte) (Product, error) {
	node, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return Product{}, err
	}

	p := Product{}

	var description string
	var text func(*html.Node) string

	text = func(n *html.Node) string {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return ""
		}

		if n.Type == html.TextNode {
			return n.Data
		}

		var b strings.Builder

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.WriteString(text(c))
			b.WriteByte(' ')
		}

		return b.String()
	}

	var visit func(*html.Node)

	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}

			if n.Data == "meta" {
				property := attrs["property"]
				if property == "" {
					property = attrs["name"]
				}
				switch property {
				case "og:title":
					p.Name = attrs["content"]
				case "og:image", "og:image:url", "og:image:secure_url":
					p.CoverURL = attrs["content"]
				case "twitter:image", "twitter:image:src":
					if p.CoverURL == "" {
						p.CoverURL = attrs["content"]
					}
				case "og:description":
					p.Description = attrs["content"]
				}
			}

			if strings.Contains(attrs["class"], "product-description") || attrs["id"] == "item-description" {
				description += text(n) + "\n"
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(node)

	p.Name = strings.TrimSpace(strings.TrimSuffix(p.Name, " - BOOTH"))
	if description != "" {
		p.Description = strings.Join(strings.Fields(description), " ")
	}

	if p.Name == "" {
		return Product{}, fmt.Errorf("Cannot identify the product name. The page may have changed or require login.")
	}

	return p, nil
}
