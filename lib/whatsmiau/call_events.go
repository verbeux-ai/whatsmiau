package whatsmiau

import (
	"fmt"
	"time"

	"github.com/verbeux-ai/whatsmiau/models"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

func (s *Whatsmiau) handleCallOfferEvent(id string, instance *models.Instance, event *events.CallOffer, eventMap map[webhookConfigEvent]bool) {
	media, isVideo := callMediaFromOffer(event.Data)
	data := newWookCallData(event.BasicCallMeta, "offer")
	data.Media = media
	data.IsVideo = isVideo
	data.RemotePlatform = event.RemotePlatform
	data.RemoteVersion = event.RemoteVersion
	s.emitCallEvent(id, instance, data, event.Timestamp, eventMap)
}

func (s *Whatsmiau) handleCallOfferNoticeEvent(id string, instance *models.Instance, event *events.CallOfferNotice, eventMap map[webhookConfigEvent]bool) {
	data := newWookCallData(event.BasicCallMeta, "offer")
	data.Media = event.Media
	data.IsGroup = data.IsGroup || event.Type == "group"
	if event.Media == "audio" || event.Media == "video" {
		isVideo := event.Media == "video"
		data.IsVideo = &isVideo
	}
	s.emitCallEvent(id, instance, data, event.Timestamp, eventMap)
}

func (s *Whatsmiau) handleCallPreAcceptEvent(id string, instance *models.Instance, event *events.CallPreAccept, eventMap map[webhookConfigEvent]bool) {
	data := newWookCallData(event.BasicCallMeta, "preaccept")
	data.RemotePlatform = event.RemotePlatform
	data.RemoteVersion = event.RemoteVersion
	s.emitCallEvent(id, instance, data, event.Timestamp, eventMap)
}

func (s *Whatsmiau) handleCallAcceptEvent(id string, instance *models.Instance, event *events.CallAccept, eventMap map[webhookConfigEvent]bool) {
	data := newWookCallData(event.BasicCallMeta, "accept")
	data.RemotePlatform = event.RemotePlatform
	data.RemoteVersion = event.RemoteVersion
	s.emitCallEvent(id, instance, data, event.Timestamp, eventMap)
}

func (s *Whatsmiau) handleCallTransportEvent(id string, instance *models.Instance, event *events.CallTransport, eventMap map[webhookConfigEvent]bool) {
	data := newWookCallData(event.BasicCallMeta, "transport")
	data.RemotePlatform = event.RemotePlatform
	data.RemoteVersion = event.RemoteVersion
	s.emitCallEvent(id, instance, data, event.Timestamp, eventMap)
}

func (s *Whatsmiau) handleCallRelayLatencyEvent(id string, instance *models.Instance, event *events.CallRelayLatency, eventMap map[webhookConfigEvent]bool) {
	s.emitCallEvent(id, instance, newWookCallData(event.BasicCallMeta, "relaylatency"), event.Timestamp, eventMap)
}

func (s *Whatsmiau) handleCallRejectEvent(id string, instance *models.Instance, event *events.CallReject, eventMap map[webhookConfigEvent]bool) {
	s.emitCallEvent(id, instance, newWookCallData(event.BasicCallMeta, "reject"), event.Timestamp, eventMap)
}

func (s *Whatsmiau) handleCallTerminateEvent(id string, instance *models.Instance, event *events.CallTerminate, eventMap map[webhookConfigEvent]bool) {
	data := newWookCallData(event.BasicCallMeta, "terminate")
	data.Reason = event.Reason
	s.emitCallEvent(id, instance, data, event.Timestamp, eventMap)
}

func (s *Whatsmiau) emitCallEvent(id string, instance *models.Instance, data *WookCallData, timestamp time.Time, eventMap map[webhookConfigEvent]bool) {
	if !eventMap[webhookConfigCall] {
		return
	}
	if timestamp.IsZero() || timestamp.Unix() <= 0 {
		timestamp = time.Now()
	}

	wookEvent := &WookEvent[WookCallData]{
		Instance: instance.ID,
		Data:     data,
		DateTime: timestamp,
		Event:    WookCall,
	}
	zap.L().Debug("call signaling event",
		zap.String("instance", id),
		zap.String("call_id", data.CallID),
		zap.String("status", data.Status),
		zap.Bool("is_group", data.IsGroup),
	)
	s.emit(wookEvent, instance.Webhook.Url)
}

func newWookCallData(meta types.BasicCallMeta, status string) *WookCallData {
	data := &WookCallData{
		CallID:            meta.CallID,
		From:              callJIDString(meta.From),
		CallCreator:       callJIDString(meta.CallCreator),
		CallCreatorAlt:    callJIDString(meta.CallCreatorAlt),
		GroupJID:          callJIDString(meta.GroupJID),
		Status:            status,
		RTCMediaAvailable: false,
	}
	data.IsGroup = data.GroupJID != ""
	return data
}

func callMediaFromOffer(node *waBinary.Node) (string, *bool) {
	if node == nil {
		return "", nil
	}
	_, isVideo := node.GetOptionalChildByTag("video")
	media := "audio"
	if isVideo {
		media = "video"
	}
	return media, &isVideo
}

func callJIDString(jid types.JID) string {
	if jid.IsEmpty() {
		return ""
	}
	return jid.String()
}

// debugCallOfferShape records only structural information from an offer
// observed through the linked client. The encrypted call key, binary bodies,
// call ID and JIDs are intentionally omitted, so it is safe for controlled
// local protocol comparisons.
func (s *Whatsmiau) debugCallOfferShape(instanceID string, node *waBinary.Node) {
	if node == nil {
		zap.L().Info("call debug offer shape", zap.String("instance", instanceID), zap.Strings("children", []string{"missing"}))
		return
	}

	shape := make([]string, 0, len(node.GetChildren()))
	for _, child := range node.GetChildren() {
		switch child.Tag {
		case "audio":
			shape = append(shape, fmt.Sprintf("audio(rate=%s)", child.AttrGetter().OptionalString("rate")))
		case "capability":
			shape = append(shape, fmt.Sprintf("capability(bytes=%d)", callNodeContentLength(child)))
		case "destination":
			shape = append(shape, fmt.Sprintf("destination(devices=%d)", len(child.GetChildren())))
		case "enc":
			shape = append(shape, "enc")
		case "device-identity":
			shape = append(shape, "device-identity")
		default:
			shape = append(shape, child.Tag)
		}
	}

	zap.L().Info("call debug offer shape", zap.String("instance", instanceID), zap.Strings("children", shape))
}

func callNodeContentLength(node waBinary.Node) int {
	content, ok := node.Content.([]byte)
	if !ok {
		return 0
	}
	return len(content)
}
