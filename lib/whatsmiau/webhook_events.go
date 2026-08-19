package whatsmiau

import "strings"

type webhookConfigEvent string

const (
	webhookConfigMessagesUpsert          webhookConfigEvent = "MESSAGES_UPSERT"
	webhookConfigMessagesUpdate          webhookConfigEvent = "MESSAGES_UPDATE"
	webhookConfigMessagesDelete          webhookConfigEvent = "MESSAGES_DELETE"
	webhookConfigMessagesSet             webhookConfigEvent = "MESSAGES_SET"
	webhookConfigContactsUpsert          webhookConfigEvent = "CONTACTS_UPSERT"
	webhookConfigGroupParticipantsUpdate webhookConfigEvent = "GROUP_PARTICIPANTS_UPDATE"
	webhookConfigConnectionUpdate        webhookConfigEvent = "CONNECTION_UPDATE"
	webhookConfigCall                    webhookConfigEvent = "CALL"
)

var webhookConfigEventReplacer = strings.NewReplacer(".", "_", "-", "_")

func normalizeWebhookConfigEvent(event string) webhookConfigEvent {
	normalized := strings.ToUpper(strings.TrimSpace(event))
	return webhookConfigEvent(webhookConfigEventReplacer.Replace(normalized))
}

func webhookEventMap(events []string) map[webhookConfigEvent]bool {
	eventMap := make(map[webhookConfigEvent]bool, len(events))
	for _, event := range events {
		normalized := normalizeWebhookConfigEvent(event)
		if normalized != "" {
			eventMap[normalized] = true
		}
	}
	return eventMap
}
