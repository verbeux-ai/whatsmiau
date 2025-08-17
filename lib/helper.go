package lib

import (
	"encoding/base64"
	"strconv"

	"go.mau.fi/whatsmeow/types"
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
