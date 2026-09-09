package whatsmiau

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

func TestBuildSendTextMessageWithLinkPreview(t *testing.T) {
	msg := buildSendTextMessage(&SendText{
		Text:            "check https://example.com/article now",
		LinkPreview:     true,
		linkPreviewInfo: &linkPreviewInfo{url: "https://example.com/article", title: "Example Article", description: "A test article", thumbnail: []byte{0xff, 0xd8}},
	}, nil, false)

	ext := msg.ExtendedTextMessage
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage for link preview")
	}
	if msg.Conversation != nil {
		t.Fatalf("expected no Conversation when link preview is present, got %v", msg.Conversation)
	}
	if ext.GetText() != "check https://example.com/article now" {
		t.Fatalf("expected full text preserved, got %q", ext.GetText())
	}
	if ext.GetMatchedText() != "https://example.com/article" {
		t.Fatalf("expected matchedText=%q, got %q", "https://example.com/article", ext.GetMatchedText())
	}
	if ext.GetTitle() != "Example Article" {
		t.Fatalf("expected title=%q, got %q", "Example Article", ext.GetTitle())
	}
	if ext.GetDescription() != "A test article" {
		t.Fatalf("expected description=%q, got %q", "A test article", ext.GetDescription())
	}
	if ext.GetPreviewType() != waE2E.ExtendedTextMessage_NONE {
		t.Fatalf("expected previewType=NONE, got %v", ext.GetPreviewType())
	}
	if len(ext.GetJPEGThumbnail()) == 0 {
		t.Fatal("expected jpegThumbnail to be attached")
	}
}

func TestBuildSendTextMessageLinkPreviewWithQuote(t *testing.T) {
	msg := buildSendTextMessage(&SendText{
		Text:        "reply https://example.com",
		LinkPreview: true,
		Quote:       &Quote{MessageID: "ABC123"},
		linkPreviewInfo: &linkPreviewInfo{
			url: "https://example.com", title: "Example", description: "Desc", thumbnail: []byte{0xff, 0xd8},
		},
	}, nil, false)

	ext := msg.ExtendedTextMessage
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage")
	}
	if ext.ContextInfo == nil || ext.ContextInfo.GetStanzaID() != "ABC123" {
		t.Fatalf("expected quote preserved in contextInfo, got %v", ext.ContextInfo)
	}
	if ext.GetMatchedText() != "https://example.com" {
		t.Fatalf("expected matchedText=%q, got %q", "https://example.com", ext.GetMatchedText())
	}
}

func TestBuildSendTextMessageLinkPreviewFalseKeepsPlainText(t *testing.T) {
	// linkPreview=false must keep the historical plain Conversation message.
	msg := buildSendTextMessage(&SendText{Text: "hello https://example.com", LinkPreview: false}, nil, false)

	if msg.Conversation == nil || msg.GetConversation() != "hello https://example.com" {
		t.Fatalf("expected Conversation=%q, got %v", "hello https://example.com", msg.Conversation)
	}
	if msg.ExtendedTextMessage != nil {
		t.Fatalf("expected no ExtendedTextMessage when preview disabled, got %v", msg.ExtendedTextMessage)
	}
}

func TestParseLinkPreviewHTML(t *testing.T) {
	body := `<html><head>
		<title>Page &amp; Title</title>
		<meta name="description" content="A meta description">
		<meta property="og:image" content="https://cdn.example.com/cover.jpg">
	</head><body>ignored</body></html>`

	title, description, imageURL := parseLinkPreviewHTML([]byte(body))
	if title != "Page & Title" {
		t.Fatalf("expected title=%q, got %q", "Page & Title", title)
	}
	if description != "A meta description" {
		t.Fatalf("expected description=%q, got %q", "A meta description", description)
	}
	if imageURL != "https://cdn.example.com/cover.jpg" {
		t.Fatalf("expected imageURL=%q, got %q", "https://cdn.example.com/cover.jpg", imageURL)
	}
}

func TestParseLinkPreviewHTMLOGTitleWinsOverTitle(t *testing.T) {
	body := `<html><head>
		<title>Fallback Title</title>
		<meta property="og:title" content="OG Title">
		<meta name="description" content="Plain description">
		<meta property="og:description" content="OG description">
	</head></html>`

	title, description, _ := parseLinkPreviewHTML([]byte(body))
	if title != "OG Title" {
		t.Fatalf("expected og:title to win, got %q", title)
	}
	if description != "OG description" {
		t.Fatalf("expected og:description to win, got %q", description)
	}
}

func TestParseLinkPreviewHTMLTrimsWhitespace(t *testing.T) {
	body := `<html><head>
		<title>  Spaced Title  </title>
		<meta property="og:description" content="   padded   ">
	</head></html>`

	title, description, _ := parseLinkPreviewHTML([]byte(body))
	if title != "Spaced Title" {
		t.Fatalf("expected trimmed title, got %q", title)
	}
	if description != "padded" {
		t.Fatalf("expected trimmed description, got %q", description)
	}
}

func TestParseLinkPreviewHTMLWithoutMetadata(t *testing.T) {
	title, description, imageURL := parseLinkPreviewHTML([]byte("<html><head><title></title></head></html>"))
	if title != "" || description != "" || imageURL != "" {
		t.Fatalf("expected empty result, got title=%q desc=%q image=%q", title, description, imageURL)
	}
}

func TestFetchLinkPreviewWithServer(t *testing.T) {
	// Pre-encode a small JPEG to serve as the og:image.
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 100, A: 255})
		}
	}
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, img, nil); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}

	const pageHTML = `<html><head><title>My Page</title>
		<meta name="description" content="The page description">
		<meta property="og:image" content="__IMG__">
	</head><body>hi</body></html>`

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "img.jpg") {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(jpegBytes.Bytes())
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(strings.Replace(pageHTML, "__IMG__", srv.URL+"/img.jpg", 1)))
	}))
	defer srv.Close()

	s := &Whatsmiau{httpClient: srv.Client()}
	info, err := s.fetchLinkPreview(context.Background(), "visit "+srv.URL+"/page.html", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected link preview info")
	}
	if info.title != "My Page" {
		t.Fatalf("expected title=%q, got %q", "My Page", info.title)
	}
	if info.description != "The page description" {
		t.Fatalf("expected description=%q, got %q", "The page description", info.description)
	}
	if !bytes.HasPrefix(info.thumbnail, []byte{0xff, 0xd8}) {
		t.Fatal("expected thumbnail to be a JPEG (FFD8 marker)")
	}
}

func TestFetchLinkPreviewWithoutURL(t *testing.T) {
	s := &Whatsmiau{httpClient: http.DefaultClient}
	info, err := s.fetchLinkPreview(context.Background(), "no links here", nil)
	if info != nil || err == nil {
		t.Fatalf("expected nil info and error for text without URL, got info=%v err=%v", info, err)
	}
}

func TestFetchLinkPreviewErrorOnUnreachableHost(t *testing.T) {
	// A fetch failure must surface as an error (the caller then sends plain text),
	// never a panic or a partial preview.
	s := &Whatsmiau{httpClient: http.DefaultClient}
	info, err := s.fetchLinkPreview(context.Background(), "https://127.0.0.1:1/unreachable", nil)
	if info != nil || err == nil {
		t.Fatalf("expected nil info and error for unreachable host, got info=%v err=%v", info, err)
	}
}

func TestTrimURLPunctuation(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://x.com.", "https://x.com"},
		{"https://x.com/a,", "https://x.com/a"},
		{"https://x.com/a?b=c", "https://x.com/a?b=c"}, // query intact
		{"https://x.com:8080/path.", "https://x.com:8080/path"},
		{"https://x.com)", "https://x.com"},     // wrapped in parens
		{"https://x.com/a]", "https://x.com/a"}, // wrapped in brackets
		{"https://x.com", "https://x.com"},
	}
	for _, c := range cases {
		if got := trimURLPunctuation(c.in); got != c.want {
			t.Errorf("trimURLPunctuation(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractURL(t *testing.T) {
	cases := []struct{ in, fetch, matched string }{
		// Explicit scheme: fetch URL == matched text, as typed.
		{"check https://x.com/a now", "https://x.com/a", "https://x.com/a"},
		{"http://x.com", "http://x.com", "http://x.com"},
		// Bare domains: https:// prepended for fetch, matched stays as typed
		// (Baileys sends matchedText exactly as the user wrote it).
		{"veja www.site.com.br/promo hoje", "https://www.site.com.br/promo", "www.site.com.br/promo"},
		{"go to example.com", "https://example.com", "example.com"},
		{"foo.bar.dev", "https://foo.bar.dev", "foo.bar.dev"},
		// Sentence punctuation after a bare domain is trimmed.
		{"open site.io.", "https://site.io", "site.io"},
		// Emails must NOT be treated as bare domains.
		{"mail me at foo@bar.com", "", ""},
		// Plain words are not URLs.
		{"no link here", "", ""},
	}
	for _, c := range cases {
		fetch, matched := extractURL(c.in)
		if fetch != c.fetch || matched != c.matched {
			t.Errorf("extractURL(%q) = (%q, %q), want (%q, %q)", c.in, fetch, matched, c.fetch, c.matched)
		}
	}
}

func TestResolveURLAbsolute(t *testing.T) {
	got := resolveURL("https://example.com/page", "https://cdn.example.com/cover.jpg")
	if got != "https://cdn.example.com/cover.jpg" {
		t.Fatalf("expected absolute URL unchanged, got %q", got)
	}
}

func TestResolveURLRelative(t *testing.T) {
	got := resolveURL("https://example.com/blog/post.html", "/img/cover.jpg")
	if got != "https://example.com/img/cover.jpg" {
		t.Fatalf("expected resolved relative URL, got %q", got)
	}
}

func TestResolveURLProtocolRelative(t *testing.T) {
	got := resolveURL("https://example.com/page", "//cdn.example.com/cover.jpg")
	if got != "https://cdn.example.com/cover.jpg" {
		t.Fatalf("expected resolved protocol-relative URL, got %q", got)
	}
}

func TestFetchLinkPreviewRelativeOGImage(t *testing.T) {
	// Serve a JPEG and a page whose og:image is protocol-relative.
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, img, nil); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "img.jpg") {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(jpegBytes.Bytes())
			return
		}
		w.Header().Set("Content-Type", "text/html")
		// protocol-relative og:image (no scheme)
		w.Write([]byte(`<html><head><title>T</title>
			<meta property="og:image" content="//` + strings.TrimPrefix(srv.URL, "http://") + `/img.jpg">
		</head></html>`))
	}))
	defer srv.Close()

	s := &Whatsmiau{httpClient: srv.Client()}
	info, err := s.fetchLinkPreview(context.Background(), "check "+srv.URL+"/page", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info.thumbnail) == 0 || !bytes.HasPrefix(info.thumbnail, []byte{0xff, 0xd8}) {
		t.Fatal("expected protocol-relative og:image to be resolved and thumbnailed")
	}
}

func TestFetchLinkPreviewTrimsTrailingPunctuation(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Page</title></head><body>ok</body></html>`))
	}))
	defer srv.Close()

	s := &Whatsmiau{httpClient: srv.Client()}
	// The trailing "." after the URL is sentence punctuation, not part of it.
	info, err := s.fetchLinkPreview(context.Background(), "see "+srv.URL+"/page.", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.url != srv.URL+"/page" {
		t.Fatalf("expected trimmed matched URL, got %q", info.url)
	}
}

func TestFetchLinkPreviewThumbnailRejectsHugeDimensions(t *testing.T) {
	// A valid JPEG header can declare huge dimensions with a tiny body; this
	// must be rejected by DecodeConfig before any large allocation happens.
	body := []byte{
		0xff, 0xd8, // SOI
		0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, // APP0
		0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, // no thumbnail
		0xff, 0xc0, 0x00, 0x0b, 0x08, // SOF0, len, precision=8
		0x00, 0x4e, 0x20, 0x00, 0x4e, 0x20, 0x01, // height=20000, width=20000, components=1
		0x01, 0x11, 0x00, // component
		0xff, 0xd9, // EOI
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(body)
	}))
	defer srv.Close()

	s := &Whatsmiau{httpClient: srv.Client()}
	thumb, _, _, _, err := s.fetchLinkPreviewThumbnail(context.Background(), srv.URL+"/huge.jpg")
	if err == nil {
		t.Fatal("expected huge-dimension image to be rejected")
	}
	if thumb != nil {
		t.Fatal("expected nil thumbnail on rejection")
	}
}

func TestFetchLinkPreviewThumbnailDecodesPNG(t *testing.T) {
	// A PNG og:image must decode and produce a JPEG thumbnail. This exercises
	// the image/png blank import registration.
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngBytes.Bytes())
	}))
	defer srv.Close()

	s := &Whatsmiau{httpClient: srv.Client()}
	thumb, original, width, height, err := s.fetchLinkPreviewThumbnail(context.Background(), srv.URL+"/cover.png")
	if err != nil {
		t.Fatalf("unexpected error decoding PNG: %v", err)
	}
	if !bytes.HasPrefix(thumb, []byte{0xff, 0xd8}) {
		t.Fatal("expected PNG source to be re-encoded as JPEG (FFD8 marker)")
	}
	// HQ payload must be the ORIGINAL bytes (PNG preserved, fiel Baileys).
	if !bytes.Equal(original, pngBytes.Bytes()) {
		t.Fatal("expected HQ payload to be the original PNG bytes")
	}
	if width != 64 || height != 64 {
		t.Fatalf("expected original dims 64x64, got %dx%d", width, height)
	}
}

func TestFetchLinkPreviewResolvesRelativeImageAgainstFinalURL(t *testing.T) {
	// A redirect (e.g. CDN/canonical) from /short to /final must make a
	// relative og:image resolve against the FINAL URL, not the typed one.
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/landing", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/article", http.StatusFound)
	})
	mux.HandleFunc("/article", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/article" && r.Method == http.MethodGet {
			// og:image is relative to /article → /img.jpg lives at /img.jpg.
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>A</title>
				<meta property="og:image" content="/img.jpg">
			</head></html>`))
			return
		}
	})
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpegBytes.Bytes())
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := &Whatsmiau{httpClient: srv.Client()}
	info, err := s.fetchLinkPreview(context.Background(), srv.URL+"/landing", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info.thumbnail) == 0 || !bytes.HasPrefix(info.thumbnail, []byte{0xff, 0xd8}) {
		t.Fatal("expected relative og:image to resolve against the post-redirect URL and be thumbnailed")
	}
}

func TestBuildSendTextMessageLinkPreviewHQFields(t *testing.T) {
	// The big-card fields (thumbnailDirectPath/mediaKey/hashes/dims) must be
	// attached to ExtendedTextMessage when the HQ upload happened.
	msg := buildSendTextMessage(&SendText{
		Text:        "https://example.com",
		LinkPreview: true,
		linkPreviewInfo: &linkPreviewInfo{
			url:         "https://example.com",
			title:       "Example",
			description: "Desc",
			thumbnail:   []byte{0xff, 0xd8},

			hqDirectPath: "/mms/thumbnail-link/abc123",
			hqMediaKey:   []byte("0123456789abcdef0123456789abcdef"),
			hqSHA256:     []byte("sha256plain"),
			hqEncSHA256:  []byte("sha256enc"),
			hqMediaKeyTs: 1725900000,
			hqWidth:      1200,
			hqHeight:     630,
		},
	}, nil, false)

	ext := msg.ExtendedTextMessage
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage")
	}
	if ext.GetThumbnailDirectPath() != "/mms/thumbnail-link/abc123" {
		t.Fatalf("expected thumbnailDirectPath, got %q", ext.GetThumbnailDirectPath())
	}
	if len(ext.GetMediaKey()) == 0 {
		t.Fatal("expected mediaKey")
	}
	if len(ext.GetThumbnailSHA256()) == 0 || len(ext.GetThumbnailEncSHA256()) == 0 {
		t.Fatal("expected thumbnail sha256 hashes")
	}
	if ext.GetMediaKeyTimestamp() != 1725900000 {
		t.Fatalf("expected mediaKeyTimestamp, got %d", ext.GetMediaKeyTimestamp())
	}
	if ext.GetThumbnailWidth() != 1200 || ext.GetThumbnailHeight() != 630 {
		t.Fatalf("expected HQ dims 1200x630, got %dx%d", ext.GetThumbnailWidth(), ext.GetThumbnailHeight())
	}
	// Title/description/text still present alongside the HQ fields.
	if ext.GetTitle() != "Example" || ext.GetMatchedText() != "https://example.com" {
		t.Fatalf("expected title+matchedText preserved, got title=%q matched=%q", ext.GetTitle(), ext.GetMatchedText())
	}
}

func TestBuildSendTextMessageLinkPreviewWithoutHQKeepsSmallCard(t *testing.T) {
	// Without an HQ upload (no og:image or upload failed) the message still has
	// the small embedded thumbnail but no thumbnailDirectPath/mediaKey.
	msg := buildSendTextMessage(&SendText{
		Text:            "https://example.com",
		LinkPreview:     true,
		linkPreviewInfo: &linkPreviewInfo{url: "https://example.com", title: "Example", thumbnail: []byte{0xff, 0xd8}},
	}, nil, false)

	ext := msg.ExtendedTextMessage
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage")
	}
	if ext.GetThumbnailDirectPath() != "" {
		t.Fatal("expected no thumbnailDirectPath without HQ upload")
	}
	if len(ext.GetMediaKey()) != 0 {
		t.Fatal("expected no mediaKey without HQ upload")
	}
	if len(ext.GetJPEGThumbnail()) == 0 {
		t.Fatal("expected small embedded thumbnail to remain")
	}
}
