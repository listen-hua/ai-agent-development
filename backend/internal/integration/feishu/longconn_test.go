package feishu

import (
	"context"
	"testing"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

type recordingLongConnectionHandler struct {
	message MessageEvent
	card    CardActionEvent
}

func (h *recordingLongConnectionHandler) HandleMessage(_ context.Context, event MessageEvent) error {
	h.message = event
	return nil
}

func (h *recordingLongConnectionHandler) HandleCardAction(_ context.Context, event CardActionEvent) (CardActionResult, error) {
	h.card = event
	return CardActionResult{ToastType: "success", ToastContent: "已处理"}, nil
}

func TestLongConnectionDispatcherConvertsMessageAndCardEvents(t *testing.T) {
	handler := &recordingLongConnectionHandler{}
	dispatcher := newLongConnectionDispatcher(handler)
	messagePayload := []byte(`{
		"schema":"2.0",
		"header":{"event_id":"evt_message","event_type":"im.message.receive_v1"},
		"event":{
			"sender":{"sender_id":{"open_id":"ou_user"}},
			"message":{"message_id":"om_message","chat_id":"oc_chat","message_type":"text","content":"{\"text\":\"@_user_1 年假怎么申请\"}","mentions":[{"key":"@_user_1"}]}
		}
	}`)
	if _, err := dispatcher.Do(context.Background(), messagePayload); err != nil {
		t.Fatal(err)
	}
	if handler.message.EventID != "evt_message" || handler.message.OpenID != "ou_user" || handler.message.ChatID != "oc_chat" {
		t.Fatalf("unexpected message event: %#v", handler.message)
	}
	if len(handler.message.MentionKeys) != 1 || handler.message.MentionKeys[0] != "@_user_1" {
		t.Fatalf("mentions were not converted: %#v", handler.message.MentionKeys)
	}

	cardPayload := []byte(`{
		"schema":"2.0",
		"header":{"event_id":"evt_card","event_type":"card.action.trigger"},
		"event":{
			"operator":{"open_id":"ou_user"},
			"context":{"open_message_id":"om_card","open_chat_id":"oc_chat"},
			"action":{"name":"confirm","value":{"approved":true}}
		}
	}`)
	response, err := dispatcher.Do(context.Background(), cardPayload)
	if err != nil {
		t.Fatal(err)
	}
	cardResponse, ok := response.(*callback.CardActionTriggerResponse)
	if !ok || cardResponse.Toast == nil || cardResponse.Toast.Content != "已处理" {
		t.Fatalf("unexpected card response: %#v", response)
	}
	if handler.card.EventID != "evt_card" || handler.card.MessageID != "om_card" || handler.card.Name != "confirm" {
		t.Fatalf("unexpected card event: %#v", handler.card)
	}
}
