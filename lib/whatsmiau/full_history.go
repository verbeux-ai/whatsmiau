package whatsmiau

import (
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"google.golang.org/protobuf/proto"
)

// init enables WhatsApp's "full history sync" handshake on all devices created
// by this package.
//
// By default whatsmeow (and Baileys) only ask for the small initial bootstrap
// that the phone is willing to send. Applications that need to import the
// maximum amount of past conversations (for example when mirroring an
// existing number into a new system) must opt in by setting RequireFullSync
// and raising the HistorySyncConfig limits before whatsmeow.NewClient is
// called. The upper bounds are gated by WhatsApp — whatsmeow only forwards
// what the server decides to send — but asking for more is harmless and lets
// big multi-device accounts receive more history.
//
// Setting it in init() ensures every call to container.NewDevice() (web pair,
// recovery, re-pair after logout) inherits the same opt-in.
func init() {
	store.DeviceProps.RequireFullSync = proto.Bool(true)
	store.DeviceProps.HistorySyncConfig = &waCompanionReg.DeviceProps_HistorySyncConfig{
		FullSyncDaysLimit:                proto.Uint32(3650),
		FullSyncSizeMbLimit:              proto.Uint32(51200),
		StorageQuotaMb:                   proto.Uint32(51200),
		InlineInitialPayloadInE2EeMsg:    proto.Bool(true),
		SupportBotUserAgentChatHistory:   proto.Bool(true),
	}
}
