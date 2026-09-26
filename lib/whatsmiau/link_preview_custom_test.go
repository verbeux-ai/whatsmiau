package whatsmiau

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func encodeTestJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height)), nil); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}
	return buf.Bytes()
}

func TestBuildCustomLinkPreviewFromImageURLSkipsPageFetch(t *testing.T) {
	// The caller already has the card data: the linked page must never be
	// requested, only the image.
	jpegBytes := encodeTestJPEG(t, 400, 300)
	var pageHits atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/product", func(w http.ResponseWriter, r *http.Request) {
		pageHits.Add(1)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Bot check</title></head></html>`))
	})
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpegBytes)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := &Whatsmiau{linkPreviewClient: srv.Client()}
	info, err := s.buildCustomLinkPreview(context.Background(), &SendText{
		Text:                   "Buy it: " + srv.URL + "/product",
		LinkPreview:            true,
		LinkPreviewImage:       srv.URL + "/img.jpg",
		LinkPreviewTitle:       "Product for $10",
		LinkPreviewDescription: "Deal of the day",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pageHits.Load() != 0 {
		t.Fatalf("expected the linked page not to be fetched, got %d hits", pageHits.Load())
	}
	if info.url != srv.URL+"/product" {
		t.Fatalf("expected matched url=%q, got %q", srv.URL+"/product", info.url)
	}
	if info.title != "Product for $10" || info.description != "Deal of the day" {
		t.Fatalf("expected caller title/description, got title=%q desc=%q", info.title, info.description)
	}
	if !bytes.HasPrefix(info.thumbnail, []byte{0xff, 0xd8}) {
		t.Fatal("expected JPEG thumbnail from the caller image")
	}
}

func TestBuildCustomLinkPreviewFromBase64(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(encodeTestJPEG(t, 64, 64))
	cases := map[string]string{
		"raw base64": encoded,
		"data URI":   "data:image/jpeg;base64," + encoded,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			s := &Whatsmiau{linkPreviewClient: http.DefaultClient}
			info, err := s.buildCustomLinkPreview(context.Background(), &SendText{
				Text:             "https://example.com/p/1",
				LinkPreview:      true,
				LinkPreviewImage: src,
				LinkPreviewTitle: "Example",
			}, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.HasPrefix(info.thumbnail, []byte{0xff, 0xd8}) {
				t.Fatal("expected JPEG thumbnail from base64 image")
			}
		})
	}
}

func TestBuildCustomLinkPreviewImageOnlyCard(t *testing.T) {
	// No title/description with an image: the card shows only the image and the
	// link domain.
	s := &Whatsmiau{linkPreviewClient: http.DefaultClient}
	data := &SendText{
		Text:             "Deal https://example.com/p/1",
		LinkPreview:      true,
		LinkPreviewImage: base64.StdEncoding.EncodeToString(encodeTestJPEG(t, 64, 64)),
	}
	info, err := s.buildCustomLinkPreview(context.Background(), data, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.title != "" || info.description != "" {
		t.Fatalf("expected empty title/description, got title=%q desc=%q", info.title, info.description)
	}
	data.linkPreviewInfo = info

	ext := buildSendTextMessage(data, nil, false).ExtendedTextMessage
	if ext == nil {
		t.Fatal("expected ExtendedTextMessage for the image-only card")
	}
	if ext.GetMatchedText() != "https://example.com/p/1" || len(ext.GetJPEGThumbnail()) == 0 {
		t.Fatalf("expected matchedText and thumbnail, got matched=%q thumb=%d bytes", ext.GetMatchedText(), len(ext.GetJPEGThumbnail()))
	}
	if ext.GetTitle() != "" {
		t.Fatalf("expected empty title, got %q", ext.GetTitle())
	}
}

func TestBuildCustomLinkPreviewTextOnlyCard(t *testing.T) {
	s := &Whatsmiau{linkPreviewClient: http.DefaultClient}
	info, err := s.buildCustomLinkPreview(context.Background(), &SendText{
		Text:             "https://example.com",
		LinkPreview:      true,
		LinkPreviewTitle: "  Example  ",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.title != "Example" || info.thumbnail != nil {
		t.Fatalf("expected trimmed title and no thumbnail, got title=%q thumb=%d bytes", info.title, len(info.thumbnail))
	}
}

func TestBuildCustomLinkPreviewImageFailureKeepsTextCard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	s := &Whatsmiau{linkPreviewClient: srv.Client()}
	info, err := s.buildCustomLinkPreview(context.Background(), &SendText{
		Text:             "https://example.com",
		LinkPreview:      true,
		LinkPreviewImage: srv.URL + "/missing.jpg",
		LinkPreviewTitle: "Example",
	}, nil)
	if err != nil {
		t.Fatalf("expected a text card when only the image fails, got error: %v", err)
	}
	if info.title != "Example" || info.thumbnail != nil {
		t.Fatalf("expected title without thumbnail, got title=%q thumb=%d bytes", info.title, len(info.thumbnail))
	}
}

func TestBuildCustomLinkPreviewImageFailureWithoutTextFails(t *testing.T) {
	// Nothing left to show: the error makes SendText fall back to plain text.
	s := &Whatsmiau{linkPreviewClient: http.DefaultClient}
	info, err := s.buildCustomLinkPreview(context.Background(), &SendText{
		Text:             "https://example.com",
		LinkPreview:      true,
		LinkPreviewImage: "not-an-image",
	}, nil)
	if err == nil || info != nil {
		t.Fatalf("expected error and nil info, got info=%v err=%v", info, err)
	}
}

func TestBuildCustomLinkPreviewWithoutURLFails(t *testing.T) {
	s := &Whatsmiau{linkPreviewClient: http.DefaultClient}
	info, err := s.buildCustomLinkPreview(context.Background(), &SendText{
		Text:             "no links here",
		LinkPreview:      true,
		LinkPreviewTitle: "Example",
	}, nil)
	if err == nil || info != nil {
		t.Fatalf("expected error for text without URL, got info=%v err=%v", info, err)
	}
}

func TestLoadLinkPreviewImageRejectsInvalidInput(t *testing.T) {
	s := &Whatsmiau{linkPreviewClient: http.DefaultClient}
	oversized := base64.StdEncoding.EncodeToString(make([]byte, maxLinkPreviewImageBytes+1))
	cases := map[string]string{
		"data URI without base64": "data:image/png,rawbytes",
		"invalid base64":          "%%%not-base64%%%",
		"oversized":               oversized,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.loadLinkPreviewImage(context.Background(), src); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadLinkPreviewImageNonASCIIDataURIHeader(t *testing.T) {
	// Regression: some runes shrink when lowercased (KELVIN SIGN is 3 bytes,
	// "k" is 1), so a comma index taken from the original string must never be
	// used on a lowercased copy. These inputs used to panic.
	s := &Whatsmiau{linkPreviewClient: http.DefaultClient}
	cases := map[string]string{
		"kelvin sign":        "data:KKK,AAAA",
		"ohm sign":           "data:ΩΩΩΩ,AAAA",
		"capital sharp s":    "data:ẞẞẞẞ,AAAA",
		"invalid utf-8":      "data:\xff\xfe\xfd,AAAA",
		"non-ascii no comma": "data:KKK",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.loadLinkPreviewImage(context.Background(), src); err == nil {
				t.Fatal("expected error for non-base64 data URI")
			}
		})
	}

	// A non-ASCII media type is still fine when the header ends in ;base64.
	encoded := base64.StdEncoding.EncodeToString(encodeTestJPEG(t, 8, 8))
	raw, err := s.loadLinkPreviewImage(context.Background(), "data:image/K;base64,"+encoded)
	if err != nil || !bytes.HasPrefix(raw, []byte{0xff, 0xd8}) {
		t.Fatalf("expected JPEG bytes, got %d bytes, err=%v", len(raw), err)
	}
}

func TestLoadLinkPreviewImageSchemeIsCaseInsensitive(t *testing.T) {
	s := &Whatsmiau{linkPreviewClient: http.DefaultClient}
	encoded := base64.StdEncoding.EncodeToString(encodeTestJPEG(t, 8, 8))
	raw, err := s.loadLinkPreviewImage(context.Background(), "DATA:IMAGE/JPEG;BASE64,"+encoded)
	if err != nil || !bytes.HasPrefix(raw, []byte{0xff, 0xd8}) {
		t.Fatalf("expected JPEG bytes from uppercase data URI, got %d bytes, err=%v", len(raw), err)
	}
}

func TestLoadLinkPreviewImageURLUsesGuardedClient(t *testing.T) {
	// Caller-supplied image URLs go through the same SSRF guard as page fetches.
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(encodeTestJPEG(t, 8, 8))
	}))
	defer srv.Close()

	s := &Whatsmiau{} // default guarded client
	_, err := s.loadLinkPreviewImage(context.Background(), srv.URL+"/img.jpg")
	if err == nil || !strings.Contains(err.Error(), "non-public address") {
		t.Fatalf("expected internal address to be refused, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("expected no request to reach the internal server, got %d", hits.Load())
	}
}

func TestSendTextLinkPreviewOptions(t *testing.T) {
	if (&SendText{LinkPreview: true}).hasCustomLinkPreview() {
		t.Fatal("expected no custom preview without linkPreview* fields")
	}
	for _, d := range []*SendText{
		{LinkPreviewImage: "x"}, {LinkPreviewTitle: "x"}, {LinkPreviewDescription: "x"},
	} {
		if !d.hasCustomLinkPreview() {
			t.Fatalf("expected custom preview for %+v", d)
		}
	}

	large, small := true, false
	if !(&SendText{}).largeLinkPreview() {
		t.Fatal("expected large card by default")
	}
	if !(&SendText{LinkPreviewLarge: &large}).largeLinkPreview() {
		t.Fatal("expected large card when linkPreviewLarge=true")
	}
	if (&SendText{LinkPreviewLarge: &small}).largeLinkPreview() {
		t.Fatal("expected small card when linkPreviewLarge=false")
	}
}
