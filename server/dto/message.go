package dto

type SendTextRequest struct {
	InstanceID       string                `param:"instance" validate:"required"`
	Number           string                `json:"number,omitempty" validate:"required"` // JID
	Text             string                `json:"text,omitempty" validate:"required"`
	Delay            int                   `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Quoted           *MessageRequestQuoted `json:"quoted,omitempty"`
	LinkPreview      bool                  `json:"linkPreview,omitempty"`
	MentionsEveryOne bool                  `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string              `json:"mentioned,omitempty"`
}

type MessageRequestQuoted struct {
	Key     QuotedKey     `json:"key,omitempty"`
	Message QuotedMessage `json:"message,omitempty"`
}

type QuotedKey struct {
	Id string `json:"id,omitempty"`
}

type QuotedMessage struct {
	Conversation string `json:"conversation,omitempty"`
}

type MessageResponseKey struct {
	RemoteJid string `json:"remoteJid,omitempty"`
	FromMe    bool   `json:"fromMe,omitempty"`
	Id        string `json:"id,omitempty"`
}

type SendTextResponse struct {
	Key              MessageResponseKey          `json:"key"`
	PushName         string                      `json:"pushName"`
	Status           string                      `json:"status"`
	Message          SendTextResponseMessage     `json:"message"`
	ContextInfo      SendTextResponseContextInfo `json:"contextInfo"`
	MessageType      string                      `json:"messageType"`
	MessageTimestamp int                         `json:"messageTimestamp"`
	InstanceId       string                      `json:"instanceId"`
	Source           string                      `json:"source"`
}

type SendTextResponseMessage struct {
	Conversation string `json:"conversation,omitempty"`
}

type SendTextResponseContextInfo struct {
	Participant   string                   `json:"participant,omitempty"`
	StanzaId      string                   `json:"stanzaId,omitempty"`
	QuotedMessage ContextInfoQuotedMessage `json:"quotedMessage,omitempty"`
}

type ContextInfoQuotedMessage struct {
	Conversation string `json:"conversation,omitempty"`
}

type SendAudioRequest struct {
	InstanceID       string                `param:"instance"`
	Number           string                `json:"number,omitempty"`
	Audio            string                `json:"audio,omitempty"`
	Delay            int                   `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Quoted           *MessageRequestQuoted `json:"quoted,omitempty"`
	MentionsEveryOne bool                  `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string              `json:"mentioned,omitempty"`
	Encoding         bool                  `json:"encoding,omitempty"`
}

type SendAudioResponseMessage struct {
	AudioMessage SendAudioResponseMessageAudio `json:"audioMessage"`
	Base64       string                        `json:"base64"`
}
type SendAudioResponseMessageAudio struct {
	DirectPath        string `json:"directPath"`
	FileEncSha256     string `json:"fileEncSha256"`
	FileLength        string `json:"fileLength"`
	FileSha256        string `json:"fileSha256"`
	MediaKey          string `json:"mediaKey"`
	MediaKeyTimestamp string `json:"mediaKeyTimestamp"`
	Mimetype          string `json:"mimetype"`
	Ptt               bool   `json:"ptt"`
	Seconds           int    `json:"seconds"`
	Url               string `json:"url"`
	Waveform          string `json:"waveform"`
}

type SendAudioResponse struct {
	ContextInfo      MessageContextInfo       `json:"contextInfo"`
	InstanceId       string                   `json:"instanceId"`
	Key              MessageResponseKey       `json:"key"`
	Message          SendAudioResponseMessage `json:"message"`
	MessageTimestamp int                      `json:"messageTimestamp"`
	MessageType      string                   `json:"messageType"`
	PushName         string                   `json:"pushName"`
	Source           string                   `json:"source"`
	Status           string                   `json:"status"`
}
type MessageContextInfo struct {
	MessageSecret             string              `json:"messageSecret"`
	DeviceListMetadata        *DeviceListMetadata `json:"deviceListMetadata,omitempty"`
	DeviceListMetadataVersion int                 `json:"deviceListMetadataVersion,omitempty"`
}

type DeviceListMetadata struct {
	SenderKeyHash      string `json:"senderKeyHash"`
	SenderTimestamp    string `json:"senderTimestamp"`
	RecipientKeyHash   string `json:"recipientKeyHash"`
	RecipientTimestamp string `json:"recipientTimestamp"`
}

type MediaType string

const (
	MediaTypeImage    MediaType = "image"
	MediaTypeVideo    MediaType = "video"
	MediaTypeDocument MediaType = "document"
)

type SendMediaRequest struct {
	Mediatype string `json:"mediatype,omitempty"`
	SendDocumentRequest
}

type SendMediaResponse struct {
	SendDocumentResponse
}

type SendDocumentRequest struct {
	InstanceID string `param:"instance"`
	Number     string `json:"number,omitempty"`
	Mimetype   string `json:"mimetype,omitempty"`
	Caption    string `json:"caption,omitempty"`
	// Media is the URL of the file
	Media            string                `json:"media,omitempty"`
	FileName         string                `json:"fileName,omitempty"`
	Delay            int                   `json:"delay,omitempty" validate:"omitempty,min=0,max=300000"`
	Quoted           *MessageRequestQuoted `json:"quoted,omitempty"`
	MentionsEveryOne bool                  `json:"mentionsEveryOne,omitempty"`
	Mentioned        []string              `json:"mentioned,omitempty"`
}

type SendDocumentResponse struct {
	Key              MessageResponseKey       `json:"key,omitempty"`
	PushName         string                   `json:"pushName,omitempty"`
	Status           string                   `json:"status,omitempty"`
	Message          SendDocumentResponseData `json:"message,omitempty"`
	ContextInfo      any                      `json:"contextInfo,omitempty"`
	MessageType      string                   `json:"messageType,omitempty"`
	MessageTimestamp int                      `json:"messageTimestamp,omitempty"`
	InstanceId       string                   `json:"instanceId,omitempty"`
	Source           string                   `json:"source,omitempty"`
}

type SendDocumentResponseData struct {
	Base64 string `json:"base64,omitempty"`
}

type SendDocumentResponseDataImage struct {
	Url               string `json:"url,omitempty"`
	Mimetype          string `json:"mimetype,omitempty"`
	Caption           string `json:"caption,omitempty"`
	FileSha256        string `json:"fileSha256,omitempty"`
	FileLength        string `json:"fileLength,omitempty"`
	Height            int    `json:"height,omitempty"`
	Width             int    `json:"width,omitempty"`
	MediaKey          string `json:"mediaKey,omitempty"`
	FileEncSha256     string `json:"fileEncSha256,omitempty"`
	DirectPath        string `json:"directPath,omitempty"`
	MediaKeyTimestamp string `json:"mediaKeyTimestamp,omitempty"`
	JpegThumbnail     string `json:"jpegThumbnail,omitempty"`
	ContextInfo       any    `json:"contextInfo,omitempty"`
}
