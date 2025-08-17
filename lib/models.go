package lib

import "time"

type Wook string

const (
	WookMessagesUpsert Wook = "messages.upsert"
	WookMessagesUpdate Wook = "messages.update"
	WookContactsUpsert Wook = "contacts.upsert"
)

type WookEvent[data any] struct {
	Instance    string    `json:"instance,omitempty"`
	Data        *data     `json:"data,omitempty"`
	Destination string    `json:"destination,omitempty"`
	DateTime    time.Time `json:"date_time,omitempty"`
	Sender      string    `json:"sender,omitempty"`
	ServerUrl   string    `json:"server_url,omitempty"`
	Apikey      string    `json:"apikey,omitempty"`
	Event       Wook      `json:"event,omitempty"`
}

type WookMessageData struct {
	Key              *WookKey                `json:"key,omitempty"`
	PushName         string                  `json:"pushName,omitempty"`
	Status           string                  `json:"status,omitempty"`
	Message          *WookMessageRaw         `json:"message,omitempty"`
	ContextInfo      *WookMessageContextInfo `json:"contextInfo,omitempty"`
	MessageType      string                  `json:"messageType,omitempty"`
	MessageTimestamp int                     `json:"messageTimestamp,omitempty"`
	InstanceId       string                  `json:"instanceId,omitempty"`
	Source           string                  `json:"source,omitempty"`
}

type WookMessageContextInfo struct {
	EphemeralSettingTimestamp             string                                 `json:"ephemeralSettingTimestamp,omitempty"`
	DisappearingMode                      *ContextInfoDisappearingMode           `json:"disappearingMode,omitempty"`
	StanzaId                              string                                 `json:"stanzaId,omitempty"`
	Participant                           string                                 `json:"participant,omitempty"`
	Expiration                            int                                    `json:"expiration,omitempty"`
	QuotedMessage                         *WookMessageContextInfoQuotedMessage   `json:"quotedMessage,omitempty"`
	MentionedJid                          []string                               `json:"mentionedJid,omitempty"`
	ConversionSource                      string                                 `json:"conversionSource,omitempty"`
	ConversionData                        string                                 `json:"conversionData,omitempty"`
	ConversionDelaySeconds                int                                    `json:"conversionDelaySeconds,omitempty"`
	WookMessageContextInfoExternalAdReply *WookMessageContextInfoExternalAdReply `json:"externalAdReply,omitempty"`
	EntryPointConversionSource            string                                 `json:"entryPointConversionSource,omitempty"`
	EntryPointConversionApp               string                                 `json:"entryPointConversionApp,omitempty"`
	EntryPointConversionDelaySeconds      int                                    `json:"entryPointConversionDelaySeconds,omitempty"`
	TrustBannerAction                     uint32                                 `json:"trustBannerAction,omitempty"`
}

type WookMessageContextInfoExternalAdReply struct {
	Title                 string `json:"title,omitempty"`
	Body                  string `json:"body,omitempty"`
	MediaType             string `json:"mediaType,omitempty"`
	ThumbnailUrl          string `json:"thumbnailUrl,omitempty"`
	Thumbnail             string `json:"thumbnail,omitempty"`
	SourceType            string `json:"sourceType,omitempty"`
	SourceId              string `json:"sourceId,omitempty"`
	SourceUrl             string `json:"sourceUrl,omitempty"`
	ContainsAutoReply     bool   `json:"containsAutoReply,omitempty"`
	RenderLargerThumbnail bool   `json:"renderLargerThumbnail,omitempty"`
	ShowAdAttribution     bool   `json:"showAdAttribution,omitempty"`
	CtwaClid              string `json:"ctwaClid,omitempty"`
}

type WookMessageContextInfoQuotedMessage struct {
	ExtendedTextMessage *struct {
		Text        string `json:"text,omitempty"`
		ContextInfo *struct {
			Expiration       int                          `json:"expiration,omitempty"`
			DisappearingMode *ContextInfoDisappearingMode `json:"disappearingMode,omitempty"`
		} `json:"contextInfo,omitempty"`
	} `json:"extendedTextMessage,omitempty"`
	ListMessage *WookListMessageRawListContextInfoQuotedMessageList `json:"listMessage,omitempty"`
}

type WookKey struct {
	RemoteJid   string `json:"remoteJid,omitempty"`
	FromMe      bool   `json:"fromMe,omitempty"`
	Id          string `json:"id,omitempty"`
	Participant string `json:"participant,omitempty"`
}

type WookMessageRaw struct {
	Conversation    string                  `json:"conversation,omitempty"`
	Base64          string                  `json:"base64,omitempty"`
	ImageMessage    *WookImageMessageRaw    `json:"imageMessage,omitempty"`
	DocumentMessage *WookDocumentMessageRaw `json:"documentMessage,omitempty"`
	AudioMessage    *WookAudioMessageRaw    `json:"audioMessage,omitempty"`
	ReactionMessage *ReactionMessageRaw     `json:"reactionMessage,omitempty"`
	//MessageContextInfo  WookMessageContextInfo `json:"messageContextInfo,omitempty"`

	ListResponseMessage *WookListMessageRaw `json:"listResponseMessage,omitempty"`
}

type WookListMessageRaw struct {
	Title             string                                   `json:"title,omitempty"`
	ListType          string                                   `json:"listType,omitempty"`
	SingleSelectReply *WookListMessageRawListSingleSelectReply `json:"singleSelectReply,omitempty"`
	ContextInfo       *WookListMessageRawListContextInfo       `json:"contextInfo,omitempty"`
	Description       string                                   `json:"description,omitempty"`
	ButtonText        string                                   `json:"buttonText,omitempty"`
	Sections          []WookListSection                        `json:"sections,omitempty"`
	FooterText        string                                   `json:"footerText,omitempty"`
}

type WookListMessageRawListSingleSelectReply struct {
	SelectedRowId string `json:"selectedRowId,omitempty"`
}

type WookListMessageRawListContextInfo struct {
	StanzaId      string                                          `json:"stanzaId,omitempty"`
	Participant   string                                          `json:"participant,omitempty"`
	QuotedMessage *WookListMessageRawListContextInfoQuotedMessage `json:"quotedMessage,omitempty"`
}

type WookListMessageRawListContextInfoQuotedMessage struct {
	MessageContextInfo *struct{}                                           `json:"messageContextInfo,omitempty"`
	ListMessage        *WookListMessageRawListContextInfoQuotedMessageList `json:"listMessage,omitempty"`
}

type WookListMessageRawListContextInfoQuotedMessageList struct {
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	ButtonText  string            `json:"buttonText,omitempty"`
	ListType    string            `json:"listType,omitempty"`
	Sections    []WookListSection `json:"sections,omitempty"`
	FooterText  string            `json:"footerText,omitempty"`
}

type WookListSection struct {
	Title string        `json:"title,omitempty"`
	Rows  []WookListRow `json:"rows,omitempty"`
}

type WookListRow struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	RowId       string `json:"rowId,omitempty"`
}

type ReactionMessageRaw struct {
	Key               *WookKey `json:"key,omitempty"`
	Text              string   `json:"text,omitempty"`
	SenderTimestampMs string   `json:"senderTimestampMs,omitempty"`
}

type WookAudioMessageRaw struct {
	Url               string           `json:"url,omitempty"`
	Mimetype          string           `json:"mimetype,omitempty"`
	FileSha256        string           `json:"fileSha256,omitempty"`
	FileLength        string           `json:"fileLength,omitempty"`
	Seconds           int              `json:"seconds,omitempty"`
	Ptt               bool             `json:"ptt,omitempty"`
	MediaKey          string           `json:"mediaKey,omitempty"`
	FileEncSha256     string           `json:"fileEncSha256,omitempty"`
	DirectPath        string           `json:"directPath,omitempty"`
	MediaKeyTimestamp string           `json:"mediaKeyTimestamp,omitempty"`
	ContextInfo       *FileContextInfo `json:"contextInfo,omitempty"`
	Waveform          string           `json:"waveform,omitempty"`
	ViewOnce          bool             `json:"viewOnce,omitempty"`
}

type WookDocumentMessageRaw struct {
	Url               string `json:"url,omitempty"`
	Mimetype          string `json:"mimetype,omitempty"`
	Title             string `json:"title,omitempty"`
	FileSha256        string `json:"fileSha256,omitempty"`
	FileLength        string `json:"fileLength,omitempty"`
	PageCount         int    `json:"pageCount,omitempty"`
	MediaKey          string `json:"mediaKey,omitempty"`
	FileName          string `json:"fileName,omitempty"`
	FileEncSha256     string `json:"fileEncSha256,omitempty"`
	DirectPath        string `json:"directPath,omitempty"`
	MediaKeyTimestamp string `json:"mediaKeyTimestamp,omitempty"`
	ContactVcard      bool   `json:"contactVcard,omitempty"`
	JpegThumbnail     string `json:"jpegThumbnail,omitempty"`
	Caption           string `json:"caption,omitempty"`
}

type WookImageMessageRaw struct {
	Url               string           `json:"url,omitempty"`
	Mimetype          string           `json:"mimetype,omitempty"`
	FileSha256        string           `json:"fileSha256,omitempty"`
	FileLength        string           `json:"fileLength,omitempty"`
	Height            int              `json:"height,omitempty"`
	Caption           string           `json:"caption,omitempty"`
	Width             int              `json:"width,omitempty"`
	MediaKey          string           `json:"mediaKey,omitempty"`
	FileEncSha256     string           `json:"fileEncSha256,omitempty"`
	DirectPath        string           `json:"directPath,omitempty"`
	MediaKeyTimestamp string           `json:"mediaKeyTimestamp,omitempty"`
	JpegThumbnail     string           `json:"jpegThumbnail,omitempty"`
	ContextInfo       *FileContextInfo `json:"contextInfo,omitempty"`
	ViewOnce          bool             `json:"viewOnce,omitempty"`
}
type FileContextInfo struct {
	DisappearingMode *ContextInfoDisappearingMode `json:"disappearingMode,omitempty"`
}

type ContextInfoDisappearingMode struct {
	Initiator     string `json:"initiator,omitempty"`
	Trigger       string `json:"trigger,omitempty"`
	InitiatedByMe bool   `json:"initiatedByMe,omitempty"`
}

type WookMessageUpdateStatus string

const (
	MessageStatusDeliveryAck WookMessageUpdateStatus = "DELIVERY_ACK"
	MessageStatusRead        WookMessageUpdateStatus = "READ"
)

type WookMessageUpdateData struct {
	MessageId   string                  `json:"messageId,omitempty"`
	KeyId       string                  `json:"keyId,omitempty"`
	RemoteJid   string                  `json:"remoteJid,omitempty"`
	FromMe      bool                    `json:"fromMe,omitempty"`
	Participant string                  `json:"participant,omitempty"`
	Status      WookMessageUpdateStatus `json:"status,omitempty"`
	InstanceId  string                  `json:"instanceId,omitempty"`
}

type WookContact struct {
	RemoteJid     string `json:"remoteJid,omitempty"`
	PushName      string `json:"pushName,omitempty"`
	ProfilePicUrl string `json:"profilePicUrl,omitempty"`
	InstanceId    string `json:"instanceId,omitempty"`
	Base64Pic     string `json:"base64Pic,omitempty"`
}

type WookContactUpsertData []WookContact
