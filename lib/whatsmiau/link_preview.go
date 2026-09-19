package whatsmiau

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"

	_ "golang.org/x/image/webp"
	_ "image/png"

	"go.mau.fi/whatsmeow"
	"go.uber.org/zap"
	"golang.org/x/image/draw"
	"golang.org/x/net/html"
)

type linkPreviewInfo struct {
	url          string
	title        string
	description  string
	thumbnail    []byte // JPEG thumbnail (~192px), nil when the page has no image
	hqDirectPath string
	hqMediaKey   []byte
	hqSHA256     []byte
	hqEncSHA256  []byte
	hqMediaKeyTs int64
	hqWidth      int
	hqHeight     int
}

const linkPreviewTimeout = 8 * time.Second
const thumbnailWidth = 192
const maxImageMegapixels = 16_000_000

var urlRegexp = regexp.MustCompile(`https?://[^\s<>"']+`)
var bareDomainRegexp = regexp.MustCompile(`(?:[a-zA-Z0-9-]+\.)+(com|net|org|io|dev|br|co|me|app|xyz|info|biz|online|site|store|tech|blog|art|gov|edu|tv|cc|ai|cloud|page|link|shop|top|pro|club|live|news|wiki|zone|fun|space|website|vip|icu|win|quest|cyou|sbs|lol|bond|cfd)[^\s<>"']*`)

func trimURLPunctuation(raw string) string {
	return strings.TrimRight(raw, `.,;:!?)]}`)
}

func extractURL(text string) (fetchURL, matchedText string) {
	trimmed := strings.TrimSpace(text)
	if matched := trimURLPunctuation(urlRegexp.FindString(trimmed)); matched != "" {
		return matched, matched
	}
	loc := bareDomainRegexp.FindStringIndex(trimmed)
	if loc == nil {
		return "", ""
	}
	// Skip matches that are part of an email address (user@domain) or preceded
	// by a word character (e.g. "foobar.com" inside a longer token).
	if loc[0] > 0 {
		prev := trimmed[loc[0]-1]
		if prev == '@' || (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || (prev >= '0' && prev <= '9') {
			return "", ""
		}
	}
	matchedText = trimURLPunctuation(trimmed[loc[0]:loc[1]])
	return "https://" + matchedText, matchedText
}

var linkPreviewDefaultClient = newLinkPreviewHTTPClient()

func isDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		switch {
		case ip4[0] == 0: // 0.0.0.0/8 "this network"
			return true
		case ip4[0] == 100 && ip4[1]&0xc0 == 64: // 100.64.0.0/10 CGNAT
			return true
		case ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0: // 192.0.0.0/24
			return true
		case ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19): // 198.18.0.0/15
			return true
		case ip4[0] >= 240: // 240.0.0.0/4 reserved
			return true
		}
	}
	return false
}

func linkPreviewDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("link preview: invalid dial address %q: %w", address, err)
	}
	if isDisallowedIP(net.ParseIP(host)) {
		return fmt.Errorf("link preview: refusing to connect to non-public address %s", address)
	}
	return nil
}

func newLinkPreviewHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   linkPreviewDialControl,
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
}

// previewHTTPClient returns the guarded client used for link preview fetches.
func (s *Whatsmiau) previewHTTPClient() *http.Client {
	if s.linkPreviewClient != nil {
		return s.linkPreviewClient
	}
	return linkPreviewDefaultClient
}

func (s *Whatsmiau) fetchLinkPreview(ctx context.Context, text string, client *whatsmeow.Client) (*linkPreviewInfo, error) {
	fetchURL, matched := extractURL(text)
	if fetchURL == "" {
		return nil, fmt.Errorf("no URL found in text")
	}

	pageCtx, cancel := context.WithTimeout(ctx, linkPreviewTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(pageCtx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; whatsmiau-link-preview/1.0)")
	// Only pages with HTML content can provide preview metadata.
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.previewHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("page returned status %d", resp.StatusCode)
	}
	if len(resp.Header.Get("Content-Type")) > 0 && !strings.Contains(resp.Header.Get("Content-Type"), "text/html") &&
		!strings.Contains(resp.Header.Get("Content-Type"), "application/xhtml+xml") {
		return nil, fmt.Errorf("page is not HTML")
	}

	// Bound the body read: link cards only need the <head> metadata.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	title, description, imageURL := parseLinkPreviewHTML(body)
	if title == "" {
		return nil, fmt.Errorf("no title found in page")
	}

	baseURL := matched
	if resp.Request.URL != nil && resp.Request.URL.String() != "" {
		baseURL = resp.Request.URL.String()
	}
	if imageURL != "" {
		imageURL = resolveURL(baseURL, imageURL)
	}

	// html.Parse already unescapes entities in attribute/text values.
	info := &linkPreviewInfo{
		url:         matched,
		title:       title,
		description: description,
	}

	if imageURL != "" {
		thumb, original, width, height, thumbErr := s.fetchLinkPreviewThumbnail(pageCtx, imageURL)
		if thumbErr != nil {
			// Image failure is non-fatal: the card still renders with text.
			return info, nil
		}
		info.thumbnail = thumb

		if client != nil && len(original) > 0 {
			if err := s.uploadLinkPreviewHQ(pageCtx, client, info, original, width, height); err != nil {
				zap.L().Warn("link preview HQ upload failed, falling back to small card", zap.Error(err))
			}
		}
	}

	return info, nil
}

func (s *Whatsmiau) uploadLinkPreviewHQ(ctx context.Context, client *whatsmeow.Client, info *linkPreviewInfo, original []byte, width, height int) error {
	if len(original) == 0 {
		return nil
	}
	uploaded, err := client.Upload(ctx, original, whatsmeow.MediaLinkThumbnail)
	if err != nil {
		return err
	}
	info.hqDirectPath = uploaded.DirectPath
	info.hqMediaKey = uploaded.MediaKey
	info.hqSHA256 = uploaded.FileSHA256
	info.hqEncSHA256 = uploaded.FileEncSHA256
	info.hqMediaKeyTs = time.Now().Unix()
	info.hqWidth = width
	info.hqHeight = height
	return nil
}

func parseLinkPreviewHTML(body []byte) (title, description, imageURL string) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", "", ""
	}

	var htmlTitle, htmlDesc, ogTitle, ogDesc string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" && htmlTitle == "" && n.FirstChild != nil {
			htmlTitle = strings.TrimSpace(n.FirstChild.Data)
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			var name, property, content string
			for _, a := range n.Attr {
				switch a.Key {
				case "name":
					name = strings.ToLower(a.Val)
				case "property":
					property = strings.ToLower(a.Val)
				case "content":
					content = a.Val
				}
			}
			content = strings.TrimSpace(content)
			switch {
			case property == "og:title" && ogTitle == "":
				ogTitle = content
			case property == "og:description" && ogDesc == "":
				ogDesc = content
			case name == "description" && htmlDesc == "" && ogDesc == "":
				htmlDesc = content
			case property == "og:image" && imageURL == "":
				imageURL = content
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	title, description = htmlTitle, htmlDesc
	if ogTitle != "" {
		title = ogTitle
	}
	if ogDesc != "" {
		description = ogDesc
	}
	return title, description, imageURL
}

func resolveURL(base, ref string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	resolved, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(resolved).String()
}

func (s *Whatsmiau) fetchLinkPreviewThumbnail(ctx context.Context, imageURL string) (thumb, original []byte, width, height int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; whatsmiau-link-preview/1.0)")

	resp, err := s.previewHTTPClient().Do(req)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, 0, 0, fmt.Errorf("image returned status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, nil, 0, 0, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, 0, 0, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxImageMegapixels {
		return nil, nil, 0, 0, fmt.Errorf("image dimensions out of bounds (%dx%d)", cfg.Width, cfg.Height)
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, 0, 0, err
	}

	bounds := src.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return nil, nil, 0, 0, fmt.Errorf("image has empty bounds")
	}

	dstW := thumbnailWidth
	dstH := bounds.Dy() * thumbnailWidth / bounds.Dx()
	if bounds.Dx() <= thumbnailWidth {
		dstW, dstH = bounds.Dx(), bounds.Dy()
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 80}); err != nil {
		return nil, nil, 0, 0, err
	}

	return buf.Bytes(), raw, bounds.Dx(), bounds.Dy(), nil
}
