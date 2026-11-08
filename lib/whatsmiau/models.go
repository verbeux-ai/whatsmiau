package whatsmiau

import (
	"time"

	"github.com/emersion/go-vcard"
)

type Wook string

const (
	WookMessagesUpsert          Wook = "messages.upsert"
	WookMessagesUpdate          Wook = "messages.update"
	WookContactsUpsert          Wook = "contacts.upsert"
	WookConnectionUpdate        Wook = "connection.update"
	WookMessagesDelete          Wook = "messages.delete"
	WookMessagesSet             Wook = "messages.set"
	WookGroupParticipantsUpdate Wook = "group-participants.update"
	WookMessagesEdited          Wook = "messages.edited"
	WookCall                    Wook = "call"
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
	IsLatest    *bool     `json:"isLatest,omitempty"`
	Progress    *int      `json:"progress,omitempty"`
}

type WookMessageData struct {
	Key              *WookKey                               `json:"key,omitempty"`
	PushName         string                                 `json:"pushName,omitempty"`
	Status           string                                 `json:"status,omitempty"`
	Message          *WookMessageRaw                        `json:"message,omitempty"`
	ContextInfo      *WookMessageContextInfo                `json:"contextInfo,omitempty"`
	Referral         *WookMessageContextInfoExternalAdReply `json:"referral,omitempty"`
	CtwaClid         string                                 `json:"ctwaClid,omitempty"`
	MessageType      string                                 `json:"messageType,omitempty"`
	MessageTimestamp int                                    `json:"messageTimestamp,omitempty"`
	InstanceId       string                                 `json:"instanceId,omitempty"`
	Source           string                                 `json:"source,omitempty"`
	PollUpdates      []WookPollUpdate                       `json:"pollUpdates,omitempty"`
}

type WookMessageContextInfo struct {
	EphemeralSettingTimestamp          string                                 `json:"ephemeralSettingTimestamp,omitempty"`
	DisappearingMode                   *ContextInfoDisappearingMode           `json:"disappearingMode,omitempty"`
	StanzaId                           string                                 `json:"stanzaId,omitempty"`
	Participant                        string                                 `json:"participant,omitempty"`
	Expiration                         int                                    `json:"expiration,omitempty"`
	QuotedMessage                      *WookMessageRaw                        `json:"quotedMessage,omitempty"`
	MentionedJid                       []string                               `json:"mentionedJid,omitempty"`
	ConversionSource                   string                                 `json:"conversionSource,omitempty"`
	ConversionData                     string                                 `json:"conversionData,omitempty"`
	ConversionDelaySeconds             *uint32                                `json:"conversionDelaySeconds,omitempty"`
	CtwaPayload                        string                                 `json:"ctwaPayload,omitempty"`
	CtwaSignals                        string                                 `json:"ctwaSignals,omitempty"`
	ExternalAdReply                    *WookMessageContextInfoExternalAdReply `json:"externalAdReply,omitempty"`
	EntryPointConversionSource         string                                 `json:"entryPointConversionSource,omitempty"`
	EntryPointConversionApp            string                                 `json:"entryPointConversionApp,omitempty"`
	EntryPointConversionDelaySeconds   *uint32                                `json:"entryPointConversionDelaySeconds,omitempty"`
	EntryPointConversionExternalSource string                                 `json:"entryPointConversionExternalSource,omitempty"`
	EntryPointConversionExternalMedium string                                 `json:"entryPointConversionExternalMedium,omitempty"`
	QuotedAd                           *WookMessageQuotedAd                   `json:"quotedAd,omitempty"`
	Utm                                *WookMessageUTMInfo                    `json:"utm,omitempty"`
	SmbClientCampaignId                string                                 `json:"smbClientCampaignId,omitempty"`
	SmbServerCampaignId                string                                 `json:"smbServerCampaignId,omitempty"`
	AlwaysShowAdAttribution            *bool                                  `json:"alwaysShowAdAttribution,omitempty"`
	TrustBannerAction                  uint32                                 `json:"trustBannerAction,omitempty"`
}

type WookMessageContextInfoExternalAdReply struct {
	Title                             string  `json:"title,omitempty"`
	Body                              string  `json:"body,omitempty"`
	MediaType                         string  `json:"mediaType,omitempty"`
	ThumbnailUrl                      string  `json:"thumbnailUrl,omitempty"`
	MediaUrl                          string  `json:"mediaUrl,omitempty"`
	Thumbnail                         string  `json:"thumbnail,omitempty"`
	SourceType                        string  `json:"sourceType,omitempty"`
	SourceId                          string  `json:"sourceId,omitempty"`
	SourceUrl                         string  `json:"sourceUrl,omitempty"`
	ContainsAutoReply                 *bool   `json:"containsAutoReply,omitempty"`
	RenderLargerThumbnail             *bool   `json:"renderLargerThumbnail,omitempty"`
	ShowAdAttribution                 *bool   `json:"showAdAttribution,omitempty"`
	CtwaClid                          string  `json:"ctwaClid,omitempty"`
	Ref                               string  `json:"ref,omitempty"`
	ClickToWhatsappCall               *bool   `json:"clickToWhatsappCall,omitempty"`
	AdContextPreviewDismissed         *bool   `json:"adContextPreviewDismissed,omitempty"`
	SourceApp                         string  `json:"sourceApp,omitempty"`
	AutomatedGreetingMessageShown     *bool   `json:"automatedGreetingMessageShown,omitempty"`
	GreetingMessageBody               string  `json:"greetingMessageBody,omitempty"`
	CtaPayload                        string  `json:"ctaPayload,omitempty"`
	DisableNudge                      *bool   `json:"disableNudge,omitempty"`
	OriginalImageUrl                  string  `json:"originalImageUrl,omitempty"`
	AutomatedGreetingMessageCtaType   string  `json:"automatedGreetingMessageCtaType,omitempty"`
	WtwaAdFormat                      *bool   `json:"wtwaAdFormat,omitempty"`
	AdType                            string  `json:"adType,omitempty"`
	WtwaWebsiteUrl                    string  `json:"wtwaWebsiteUrl,omitempty"`
	AdPreviewUrl                      string  `json:"adPreviewUrl,omitempty"`
	ContainsCtwaFlowsAutoReply        *bool   `json:"containsCtwaFlowsAutoReply,omitempty"`
	AgmThumbnailStrategy              *int32  `json:"agmThumbnailStrategy,omitempty"`
	AgmTitleStrategy                  *int32  `json:"agmTitleStrategy,omitempty"`
	AgmSubtitleStrategy               *int32  `json:"agmSubtitleStrategy,omitempty"`
	AgmHeaderInteractionStrategy      *int32  `json:"agmHeaderInteractionStrategy,omitempty"`
	ContainsCtwaFlowsAutoLabel        *bool   `json:"containsCtwaFlowsAutoLabel,omitempty"`
	ProductId                         string  `json:"productId,omitempty"`
	ContainsCtwaPromo                 *bool   `json:"containsCtwaPromo,omitempty"`
	CtwaPromoResponseId               string  `json:"ctwaPromoResponseId,omitempty"`
	CtwaPromoOfferId                  string  `json:"ctwaPromoOfferId,omitempty"`
	CtwaPromoAdEntryId                string  `json:"ctwaPromoAdEntryId,omitempty"`
	CtwaPromoResponseExpiresAtSeconds *uint64 `json:"ctwaPromoResponseExpiresAtSeconds,omitempty"`
	CtwaPromoSchemaVersion            *uint32 `json:"ctwaPromoSchemaVersion,omitempty"`
	CtwaPromoAdgroupId                string  `json:"ctwaPromoAdgroupId,omitempty"`
}

type WookMessageQuotedAd struct {
	AdvertiserName string `json:"advertiserName,omitempty"`
	MediaType      string `json:"mediaType,omitempty"`
	JpegThumbnail  string `json:"jpegThumbnail,omitempty"`
	Caption        string `json:"caption,omitempty"`
}

type WookMessageUTMInfo struct {
	UtmSource   string `json:"utmSource,omitempty"`
	UtmCampaign string `json:"utmCampaign,omitempty"`
}

type WookMessageExtendedTextMessage struct {
	Text        string                                     `json:"text,omitempty"`
	ContextInfo *WookMessageExtendedTextMessageContextInfo `json:"contextInfo,omitempty"`
}

type WookMessageExtendedTextMessageContextInfo struct {
	Expiration       int                          `json:"expiration,omitempty"`
	DisappearingMode *ContextInfoDisappearingMode `json:"disappearingMode,omitempty"`
}

type WookKey struct {
	RemoteJid      string `json:"remoteJid,omitempty"`
	RemoteLid      string `json:"remoteLid,omitempty"`
	FromMe         bool   `json:"fromMe,omitempty"`
	Id             string `json:"id,omitempty"`
	Participant    string `json:"participant,omitempty"`
	AddressingMode string `json:"addressingMode,omitempty"`
}

type WookMessageRaw struct {
	Conversation         string                   `json:"conversation,omitempty"`
	Base64               string                   `json:"base64,omitempty"`
	ImageMessage         *WookImageMessageRaw     `json:"imageMessage,omitempty"`
	DocumentMessage      *WookDocumentMessageRaw  `json:"documentMessage,omitempty"`
	VideoMessage         *WookVideoMessageRaw     `json:"videoMessage,omitempty"`
	AudioMessage         *WookAudioMessageRaw     `json:"audioMessage,omitempty"`
	ReactionMessage      *ReactionMessageRaw      `json:"reactionMessage,omitempty"`
	ContactMessage       *ContactMessageRaw       `json:"contactMessage,omitempty"`
	ContactsArrayMessage *ContactsArrayMessageRaw `json:"contactsArrayMessage,omitempty"`
	//MessageContextInfo  WookMessageContextInfo `json:"messageContextInfo,omitempty"`

	ListResponseMessage    *WookListMessageRaw            `json:"listResponseMessage,omitempty"`
	ButtonsResponseMessage *WookButtonsResponseMessageRaw `json:"buttonsResponseMessage,omitempty"`
	InteractiveMessage     *WookInteractiveMessageRaw     `json:"interactiveMessage,omitempty"`
	StickerMessage         *WookStickerMessageRaw         `json:"stickerMessage,omitempty"`
	LocationMessage        *WookLocationMessageRaw        `json:"locationMessage,omitempty"`
	LiveLocationMessage    *WookLiveLocationMessageRaw    `json:"liveLocationMessage,omitempty"`
	PollCreationMessage    *WookPollCreationMessageRaw    `json:"pollCreationMessage,omitempty"`
	PollUpdateMessage      *WookPollUpdateMessageRaw      `json:"pollUpdateMessage,omitempty"`
	PtvMessage             *WookPtvMessageRaw             `json:"ptvMessage,omitempty"`
	EncCommentMessage      *WookEncCommentMessageRaw      `json:"encCommentMessage,omitempty"`
	MediaURL               string                         `json:"mediaUrl,omitempty"` // Sent when connect with some storage
}

type ContactsArrayMessageRaw struct {
	DisplayName string              `json:"displayName,omitempty"`
	Contacts    []ContactMessageRaw `json:"contacts,omitempty"`
}

type ContactMessageRaw struct {
	VCard        string     `json:"vcard,omitempty"`
	DisplayName  string     `json:"displayName,omitempty"`
	DecodedVcard vcard.Card `json:"decodedVcard,omitempty" swaggerignore:"true"`
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

type WookButtonsResponseMessageRaw struct {
	SelectedButtonID    string `json:"selectedButtonId,omitempty"`
	SelectedDisplayText string `json:"selectedDisplayText,omitempty"`
}

type WookInteractiveMessageRaw struct {
	Header            *WookInteractiveMessageHeaderRaw `json:"header,omitempty"`
	Body              *WookInteractiveMessageBodyRaw   `json:"body,omitempty"`
	Footer            *WookInteractiveMessageFooterRaw `json:"footer,omitempty"`
	NativeFlowMessage *WookNativeFlowMessageRaw        `json:"nativeFlowMessage,omitempty"`
}

type WookInteractiveMessageHeaderRaw struct {
	Title string `json:"title,omitempty"`
}

type WookInteractiveMessageBodyRaw struct {
	Text string `json:"text,omitempty"`
}

type WookInteractiveMessageFooterRaw struct {
	Text string `json:"text,omitempty"`
}

type WookNativeFlowMessageRaw struct {
	MessageVersion int32                     `json:"messageVersion,omitempty"`
	Buttons        []WookNativeFlowButtonRaw `json:"buttons,omitempty"`
}

type WookNativeFlowButtonRaw struct {
	Name             string `json:"name,omitempty"`
	ButtonParamsJSON string `json:"buttonParamsJson,omitempty"`
}

type WookListMessageRawListSingleSelectReply struct {
	SelectedRowId string `json:"selectedRowId,omitempty"`
}

type WookListMessageRawListContextInfo struct {
	StanzaId      string                                    `json:"stanzaId,omitempty"`
	Participant   string                                    `json:"participant,omitempty"`
	QuotedMessage *WookListMessageRawListContextInfoMessage `json:"quotedMessage,omitempty"`
}

type WookListMessageRawListContextInfoMessage struct {
	ListMessage *WookListMessageRawListContextInfoMessageList `json:"listMessage,omitempty"`
}

type WookListMessageRawListContextInfoMessageList struct {
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

type WookEncCommentMessageRaw struct {
	TargetMessageKey *WookKey `json:"targetMessageKey,omitempty"`
	EncPayload       string   `json:"encPayload,omitempty"`
	EncIv            string   `json:"encIv,omitempty"`
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

type WookVideoMessageRaw struct {
	Url           string `json:"url,omitempty"`
	Mimetype      string `json:"mimetype,omitempty"`
	Caption       string `json:"caption,omitempty"`
	FileSha256    string `json:"fileSha256,omitempty"`
	FileLength    string `json:"fileLength,omitempty"`
	Seconds       uint32 `json:"seconds,omitempty"`
	MediaKey      string `json:"mediaKey,omitempty"`
	FileEncSha256 string `json:"fileEncSha256,omitempty"`
	JPEGThumbnail string `json:"jpegThumbnail,omitempty"`
	GIFPlayback   bool   `json:"gifPlayback,omitempty"`
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

type WookMessageDeleteData struct {
	Id          string `json:"id,omitempty"`
	RemoteJid   string `json:"remoteJid,omitempty"`
	FromMe      bool   `json:"fromMe"`
	Participant string `json:"participant,omitempty"`
	Status      string `json:"status,omitempty"`
	InstanceId  string `json:"instanceId,omitempty"`
}

type WookMessageUpdateData struct {
	MessageId      string                  `json:"messageId,omitempty"`
	KeyId          string                  `json:"keyId,omitempty"`
	RemoteJid      string                  `json:"remoteJid,omitempty"`
	RemoteLid      string                  `json:"remoteLid"`
	FromMe         bool                    `json:"fromMe,omitempty"`
	Participant    string                  `json:"participant,omitempty"`
	ParticipantLid string                  `json:"participantLid,omitempty"`
	Status         WookMessageUpdateStatus `json:"status,omitempty"`
	InstanceId     string                  `json:"instanceId,omitempty"`
}

type WookMessageEditData struct {
	Key              *WookKey        `json:"key,omitempty"`
	EditedMessage    *WookMessageRaw `json:"editedMessage,omitempty"`
	MessageType      string          `json:"messageType,omitempty"`
	InstanceId       string          `json:"instanceId,omitempty"`
	PushName         string          `json:"pushName,omitempty"`
	Status           string          `json:"status,omitempty"`
	MessageTimestamp int             `json:"messageTimestamp,omitempty"`
	Source           string          `json:"source,omitempty"`
}

type WookContact struct {
	RemoteJid     string `json:"remoteJid,omitempty"`
	RemoteLid     string `json:"remoteLid"`
	PushName      string `json:"pushName,omitempty"`
	ProfilePicUrl string `json:"profilePicUrl,omitempty"`
	InstanceId    string `json:"instanceId,omitempty"`
	Base64Pic     string `json:"base64Pic,omitempty"`
}

type WookContactUpsertData []WookContact

type WookConnectionUpdateData struct {
	Instance          string `json:"instance,omitempty"`
	Wuid              string `json:"wuid,omitempty"`
	ProfileName       string `json:"profileName,omitempty"`
	ProfilePictureUrl string `json:"profilePictureUrl,omitempty"`
	State             string `json:"state"`
	StatusReason      int    `json:"statusReason,omitempty"`
}

// WookCallData contains call signaling metadata only. It intentionally omits
// binary call nodes, session keys and media data.
type WookCallData struct {
	CallID            string `json:"callId,omitempty"`
	From              string `json:"from,omitempty"`
	CallCreator       string `json:"callCreator,omitempty"`
	CallCreatorAlt    string `json:"callCreatorAlt,omitempty"`
	GroupJID          string `json:"groupJid,omitempty"`
	Status            string `json:"status,omitempty"`
	Media             string `json:"media,omitempty"`
	IsVideo           *bool  `json:"isVideo,omitempty"`
	IsGroup           bool   `json:"isGroup,omitempty"`
	RemotePlatform    string `json:"remotePlatform,omitempty"`
	RemoteVersion     string `json:"remoteVersion,omitempty"`
	Reason            string `json:"reason,omitempty"`
	RTCMediaAvailable bool   `json:"rtcMediaAvailable"`
}

type WookGroupParticipantsUpdateData struct {
	ID               string                          `json:"id,omitempty"`
	Author           string                          `json:"author,omitempty"`
	Participants     []WookGroupParticipantJID       `json:"participants,omitempty"`
	Action           string                          `json:"action,omitempty"`
	ParticipantsData []WookGroupParticipantsDataItem `json:"participantsData,omitempty"`
}

type WookGroupParticipantJID struct {
	ID          string `json:"id,omitempty"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
	Admin       *bool  `json:"admin,omitempty"`
}

type WookGroupParticipantsDataItem struct {
	JID         WookGroupParticipantJID `json:"jid,omitempty"`
	PhoneNumber string                  `json:"phoneNumber,omitempty"`
}
