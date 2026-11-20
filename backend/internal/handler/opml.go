package handler

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"news_panel/internal/pkg/httpc"
	"news_panel/internal/pull"
)

// maxOPMLBytes bounds remote OPML downloads to protect tiny VPS memory budgets.
const maxOPMLBytes int64 = 2 << 20

// rewriteGitHubBlobURL rewrites a GitHub blob file URL such as
// https://github.com/<owner>/<repo>/blob/<ref>/<path> into its raw content URL
// https://raw.githubusercontent.com/<owner>/<repo>/<ref>/<path>.
func rewriteGitHubBlobURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") {
		return "", false
	}
	escapedPath := strings.Trim(parsed.EscapedPath(), "/")
	parts := strings.Split(escapedPath, "/")
	if len(parts) < 5 || parts[0] == "" || parts[1] == "" || parts[2] != "blob" || parts[3] == "" {
		return "", false
	}
	rest := strings.Join(parts[4:], "/")
	if strings.TrimSpace(rest) == "" {
		return "", false
	}
	rewritten := "https://raw.githubusercontent.com/" + parts[0] + "/" + parts[1] + "/" + parts[3] + "/" + rest
	return rewritten, true
}

type opmlDocument struct {
	Body opmlBody `xml:"body"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Attrs    []xml.Attr    `xml:",any,attr"`
	Outlines []opmlOutline `xml:"outline"`
}

// parseOPMLDocument parses an XML payload if its root element is <opml>.
// Returns (feeds, true) when the document is valid OPML, or (nil, false) otherwise.
func parseOPMLDocument(data []byte) ([]discoveredFeed, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, false
	}

	decoder := xml.NewDecoder(bytes.NewReader(trimmed))
	var root xml.StartElement
	foundRoot := false
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, false
		}
		if se, ok := tok.(xml.StartElement); ok {
			root = se
			foundRoot = true
			break
		}
	}
	if !foundRoot || !strings.EqualFold(root.Name.Local, "opml") {
		return nil, false
	}

	var doc opmlDocument
	if err := decoder.DecodeElement(&doc, &root); err != nil {
		return nil, false
	}

	feeds := make([]discoveredFeed, 0)
	seen := make(map[string]struct{})

	var walk func(outlines []opmlOutline, groupName string)
	walk = func(outlines []opmlOutline, groupName string) {
		for _, outline := range outlines {
			var xmlURL, htmlURL, titleAttr, textAttr string
			for _, attr := range outline.Attrs {
				switch strings.ToLower(attr.Name.Local) {
				case "xmlurl":
					xmlURL = strings.TrimSpace(attr.Value)
				case "htmlurl":
					htmlURL = strings.TrimSpace(attr.Value)
				case "title":
					titleAttr = strings.TrimSpace(attr.Value)
				case "text":
					textAttr = strings.TrimSpace(attr.Value)
				}
			}

			label := titleAttr
			if label == "" {
				label = textAttr
			}

			if xmlURL != "" {
				if _, exists := seen[xmlURL]; !exists {
					seen[xmlURL] = struct{}{}
					title := pull.NormalizeTitle(label)
					if title == "" {
						title = xmlURL
					}
					feeds = append(feeds, discoveredFeed{
						Title:     title,
						Link:      xmlURL,
						SiteURL:   htmlURL,
						GroupName: groupName,
					})
				}
				if len(outline.Outlines) > 0 {
					walk(outline.Outlines, groupName)
				}
				continue
			}

			nextGroup := groupName
			if label != "" {
				nextGroup = label
			}
			if len(outline.Outlines) > 0 {
				walk(outline.Outlines, nextGroup)
			}
		}
	}

	walk(doc.Body.Outlines, "")
	return feeds, true
}

func isAllowedDiscoveredFeedURL(rawURL string, allowPrivate bool) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return false
	}
	if allowPrivate {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() {
			return false
		}
	}
	return true
}

func (h *Handler) tryFetchOPML(ctx context.Context, target, proxy string, allowPrivateFeeds bool) ([]discoveredFeed, bool, error) {
	if err := httpc.ValidateRequestURL(ctx, target, allowPrivateFeeds); err != nil {
		return nil, false, err
	}

	client, err := httpc.NewClient(30*time.Second, proxy, allowPrivateFeeds)
	if err != nil {
		return nil, false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, false, err
	}
	httpc.SetDefaultHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, errors.New("remote opml fetch failed")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOPMLBytes+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) > maxOPMLBytes {
		return nil, false, errors.New("opml payload exceeds 2 MiB")
	}

	feeds, isOPML := parseOPMLDocument(body)
	if !isOPML {
		return nil, false, nil
	}

	filtered := make([]discoveredFeed, 0, len(feeds))
	for _, feed := range feeds {
		if isAllowedDiscoveredFeedURL(feed.Link, allowPrivateFeeds) {
			filtered = append(filtered, feed)
		}
	}
	return filtered, true, nil
}
