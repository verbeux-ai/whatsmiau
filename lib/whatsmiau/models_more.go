package whatsmiau

import "time"

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
	// PollType and CorrectAnswer are set for channel quizzes
	// (pollCreationMessageV5).
	PollType      string          `json:"pollType,omitempty"`
	CorrectAnswer *WookPollOption `json:"correctAnswer,omitempty"`
}

type WookPollVote struct {
	EncPayload      string   `json:"encPayload,omitempty"`
	EncIv           string   `json:"encIv,omitempty"`
	SelectedOptions []string `json:"selectedOptions,omitempty"`
}

type WookPollUpdateMessageRaw struct {
	PollCreationMessageKey *WookKey      `json:"pollCreationMessageKey,omitempty"`
	Vote                   *WookPollVote `json:"vote,omitempty"`
	SenderTimestampMs      string        `json:"senderTimestampMs,omitempty"`
	// VoteCounts is set for channel polls and quizzes, whose votes are
	// anonymous: the total for every option.
	VoteCounts []WookPollVoteCount `json:"voteCounts,omitempty"`
}

type WookPollVoteCount struct {
	OptionName string `json:"optionName,omitempty"`
	// OptionHash is the hex SHA-256 of the option name, as WhatsApp sends it.
	OptionHash string `json:"optionHash"`
	Count      int    `json:"count"`
}

// WookQuestionResponseMessageRaw reports the number of private responses to
// a channel question. WhatsApp does not send the responses themselves to
// linked devices.
type WookQuestionResponseMessageRaw struct {
	Key            *WookKey `json:"key,omitempty"`
	ResponsesCount int      `json:"responsesCount"`
}

type WookPollUpdate struct {
	Name   string   `json:"name,omitempty"`
	Voters []string `json:"voters,omitempty"`
}

type PollOptionHash struct {
	Name string   `json:"name"`
	Hash [32]byte `json:"hash"`
}

type PollCreationEntry struct {
	Options   []PollOptionHash `json:"options"`
	CreatedAt time.Time        `json:"createdAt"`
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
