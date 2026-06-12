package whatsmiau

type WookStickerMessageRaw struct {
	Url               string `json:"url,omitempty"`
	FileSha256        string `json:"fileSha256,omitempty"`
	FileEncSha256     string `json:"fileEncSha256,omitempty"`
	MediaKey          string `json:"mediaKey,omitempty"`
	Mimetype          string `json:"mimetype,omitempty"`
	DirectPath        string `json:"directPath,omitempty"`
	FileLength        string `json:"fileLength,omitempty"`
	MediaKeyTimestamp string `json:"mediaKeyTimestamp,omitempty"`
	IsAnimated        bool   `json:"isAnimated,omitempty"`
	PngThumbnail      string `json:"pngThumbnail,omitempty"`
	Height            int    `json:"height,omitempty"`
	Width             int    `json:"width,omitempty"`
}

type WookLocationMessageRaw struct {
	DegreesLatitude  float64 `json:"degreesLatitude,omitempty"`
	DegreesLongitude float64 `json:"degreesLongitude,omitempty"`
	Name             string  `json:"name,omitempty"`
	Address          string  `json:"address,omitempty"`
	Url              string  `json:"url,omitempty"`
	JpegThumbnail    string  `json:"jpegThumbnail,omitempty"`
}

type WookLiveLocationMessageRaw struct {
	DegreesLatitude              float64 `json:"degreesLatitude,omitempty"`
	DegreesLongitude             float64 `json:"degreesLongitude,omitempty"`
	AccuracyInMeters             uint32  `json:"accuracyInMeters,omitempty"`
	SpeedInMps                   float32 `json:"speedInMps,omitempty"`
	DegreesClockwiseFromMagNorth uint32  `json:"degreesClockwiseFromMagneticNorth,omitempty"`
	Caption                      string  `json:"caption,omitempty"`
	SequenceNumber               int64   `json:"sequenceNumber,omitempty"`
	TimeOffset                   uint32  `json:"timeOffset,omitempty"`
	JpegThumbnail                string  `json:"jpegThumbnail,omitempty"`
}

type WookPollOption struct {
	OptionName string `json:"optionName,omitempty"`
}

type WookPollCreationMessageRaw struct {
	Name                   string           `json:"name,omitempty"`
	Options                []WookPollOption `json:"options,omitempty"`
	SelectableOptionsCount uint32           `json:"selectableOptionsCount,omitempty"`
}

type WookPollUpdateMessageRaw struct {
	PollCreationMessageKey *WookKey `json:"pollCreationMessageKey,omitempty"`
	SenderTimestampMs      string   `json:"senderTimestampMs,omitempty"`
	EncPayload             string   `json:"encPayload,omitempty"`
	EncIv                  string   `json:"encIv,omitempty"`
}

type WookPtvMessageRaw struct {
	Url           string `json:"url,omitempty"`
	Mimetype      string `json:"mimetype,omitempty"`
	FileSha256    string `json:"fileSha256,omitempty"`
	FileLength    string `json:"fileLength,omitempty"`
	Seconds       uint32 `json:"seconds,omitempty"`
	MediaKey      string `json:"mediaKey,omitempty"`
	FileEncSha256 string `json:"fileEncSha256,omitempty"`
	DirectPath    string `json:"directPath,omitempty"`
	JpegThumbnail string `json:"jpegThumbnail,omitempty"`
}
