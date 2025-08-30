package whatsmiau

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

func b64(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func u64(n uint64) string {
	return strconv.FormatUint(n, 10)
}

func i64(n int64) string {
	return strconv.FormatInt(n, 10)
}

func jids(j types.JID) string {
	if j.User == "" && j.Server == "" {
		return ""
	}
	return j.String()
}

func (s *Whatsmiau) getCtx(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Returns audioConverted, waveform, duration and an error
func convertAudio(data []byte, bars int) ([]byte, []byte, float64, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, nil, 0, errors.New("ffmpeg not found in path (install to decode .ogg opus/vorbis)")
	}

	tempIn, err := os.CreateTemp("", "audio-*.ogg")
	if err != nil {
		return nil, nil, 0, err
	}
	defer os.Remove(tempIn.Name())
	if _, err := io.Copy(tempIn, bytes.NewReader(data)); err != nil {
		return nil, nil, 0, err
	}
	if err := tempIn.Close(); err != nil {
		return nil, nil, 0, err
	}

	out, err := exec.Command(
		"ffmpeg",
		"-i", tempIn.Name(),
		"-ac", "1",
		"-ar", "48000",
		"-f", "s16le",
		"-hide_banner",
		"-loglevel", "error",
		"pipe:1",
	).Output()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed running ffmpeg: %w", err)
	}
	if len(out) < 2 {
		return nil, nil, 0, errors.New("no audio data after decoding")
	}

	// Also convert to Ogg/Opus for stable playback/sharing
	oggOut, err := exec.Command(
		"ffmpeg",
		"-i", tempIn.Name(),
		"-vn",
		"-c:a", "libopus",
		"-b:a", "64k",
		"-f", "ogg",
		"-hide_banner",
		"-loglevel", "error",
		"pipe:1",
	).Output()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed converting to ogg opus: %w", err)
	}
	if len(oggOut) == 0 {
		return nil, nil, 0, errors.New("no data after opus conversion")
	}

	const sampleRate = 48000.0
	n := len(out) / 2
	durationSec := float64(n) / sampleRate

	samples := make([]int16, n)
	for i := 0; i < n; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(out[2*i : 2*i+2]))
	}

	values := rmsByBars(samples, bars)

	scale := percentile(values, 0.98)
	if scale <= 0 {
		for _, v := range values {
			if v > scale {
				scale = v
			}
		}
	}
	if scale == 0 {
		return oggOut, make([]byte, len(values)), durationSec, nil
	}

	buf := make([]byte, len(values))
	for i, v := range values {
		x := (v / scale) * 255.0
		if x < 0 {
			x = 0
		}
		if x > 255 {
			x = 255
		}
		buf[i] = byte(math.Round(x))
	}

	return oggOut, buf, durationSec, nil
}

func rmsByBars(samples []int16, bars int) []float64 {
	if bars < 1 {
		bars = 1
	}
	total := len(samples)
	if total == 0 {
		return make([]float64, bars)
	}
	seg := total / bars
	if seg == 0 {
		seg = 1
	}

	values := make([]float64, 0, bars)
	for i := 0; i < bars; i++ {
		start := i * seg
		end := start + seg
		if i == bars-1 || end > total {
			end = total
		}
		if start >= end {
			values = append(values, 0)
			continue
		}

		var sumSq float64
		for _, s := range samples[start:end] {
			f := float64(s) / 32768.0
			sumSq += f * f
		}
		rms := math.Sqrt(sumSq / float64(end-start))
		values = append(values, rms)
	}
	return values
}

func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}
	idx := int(math.Ceil(p*float64(len(cp)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func extractMimetype(decodedData []byte, fileName string) (string, error) {
	ext := filepath.Ext(fileName)
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType, nil
		}
	}

	var dataSample []byte
	if len(decodedData) > 512 {
		dataSample = decodedData[:512]
	} else {
		dataSample = decodedData
	}
	detected := http.DetectContentType(dataSample)
	return detected, nil
}

func extractExtFromFile(fileName, mimeType string, file *os.File) string {
	ext := filepath.Ext(fileName)
	if ext == "" {
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			if len(exts) > 1 {
				return exts[1]
			} else {
				ext = exts[0]
			}
		} else {
			buf := make([]byte, 512)
			n, err := file.Read(buf)
			if err != nil && err != io.EOF {
				zap.L().Error("failed to read file", zap.Error(err))
			}
			detected := http.DetectContentType(buf[:n])
			if exts, _ := mime.ExtensionsByType(detected); len(exts) > 0 {
				ext = exts[0]
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				zap.L().Error("failed to seek image", zap.Error(err))
			}
		}
	}

	return strings.TrimPrefix(ext, ".")
}
