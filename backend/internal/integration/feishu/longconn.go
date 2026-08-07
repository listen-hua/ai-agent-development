package feishu

import (
	"context"
	"errors"
	"log/slog"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkmeetingroom "github.com/larksuite/oapi-sdk-go/v3/service/meeting_room/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type MessageEvent struct {
	EventID     string
	OpenID      string
	ChatID      string
	MessageID   string
	MessageType string
	Content     string
	MentionKeys []string
}

type CardActionEvent struct {
	EventID    string
	OpenID     string
	ChatID     string
	MessageID  string
	Name       string
	Value      map[string]any
	FormValue  map[string]any
	Options    []string
	Option     string
	InputValue string
}

type CardActionResult struct {
	ToastType    string
	ToastContent string
	Card         any
}

type ContactChangeEvent struct {
	EventID string
	OpenID  string
	Change  string
}

type LongConnectionHandler interface {
	HandleMessage(context.Context, MessageEvent) error
	HandleCardAction(context.Context, CardActionEvent) (CardActionResult, error)
}

type ContactChangeHandler interface {
	HandleContactChange(context.Context, ContactChangeEvent) error
}

type MeetingRoomStatusChangeEvent struct {
	EventID string
	RoomID  string
	Name    string
}

type MeetingRoomStatusChangeHandler interface {
	HandleMeetingRoomStatusChange(context.Context, MeetingRoomStatusChangeEvent) error
}

type LongConnection struct {
	configured bool
	client     *larkws.Client
}

func NewLongConnection(appID, appSecret string, handler LongConnectionHandler, features ...any) *LongConnection {
	configured := appID != "" && appSecret != "" && handler != nil
	connection := &LongConnection{configured: configured}
	if !configured {
		return connection
	}
	var contactHandler ContactChangeHandler
	var meetingRoomHandler MeetingRoomStatusChangeHandler
	for _, feature := range features {
		if value, ok := feature.(ContactChangeHandler); ok {
			contactHandler = value
		}
		if value, ok := feature.(MeetingRoomStatusChangeHandler); ok {
			meetingRoomHandler = value
		}
	}
	eventHandler := newLongConnectionDispatcher(handler, contactHandler, meetingRoomHandler)
	connection.client = larkws.NewClient(appID, appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelWarn),
		larkws.WithOnReady(func() { slog.Info("feishu long connection ready") }),
		larkws.WithOnReconnecting(func() { slog.Warn("feishu long connection reconnecting") }),
		larkws.WithOnReconnected(func() { slog.Info("feishu long connection reconnected") }),
		larkws.WithOnDisconnected(func() { slog.Warn("feishu long connection disconnected") }),
		larkws.WithOnError(func(err error) { slog.Error("feishu long connection error", "error", err) }),
	)
	return connection
}

func (c *LongConnection) Configured() bool { return c != nil && c.configured }

func (c *LongConnection) Start(ctx context.Context) error {
	if !c.Configured() {
		return errors.New("feishu long connection is not configured")
	}
	return c.client.Start(ctx)
}

func (c *LongConnection) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

func newLongConnectionDispatcher(handler LongConnectionHandler, features ...any) *dispatcher.EventDispatcher {
	eventDispatcher := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			return handler.HandleMessage(ctx, toMessageEvent(event))
		}).
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			result, err := handler.HandleCardAction(ctx, toCardActionEvent(event))
			if err != nil {
				return nil, err
			}
			response := &callback.CardActionTriggerResponse{}
			if result.ToastContent != "" {
				response.Toast = &callback.Toast{Type: result.ToastType, Content: result.ToastContent}
			}
			if result.Card != nil {
				response.Card = &callback.Card{Type: "raw", Data: result.Card}
			}
			return response, nil
		})
	var contactHandler ContactChangeHandler
	var meetingRoomHandler MeetingRoomStatusChangeHandler
	for _, feature := range features {
		if value, ok := feature.(ContactChangeHandler); ok {
			contactHandler = value
		}
		if value, ok := feature.(MeetingRoomStatusChangeHandler); ok {
			meetingRoomHandler = value
		}
	}
	if contactHandler != nil {
		eventDispatcher = eventDispatcher.OnP2UserCreatedV3(func(ctx context.Context, event *larkcontact.P2UserCreatedV3) error {
			return contactHandler.HandleContactChange(ctx, toContactChangeEvent(event.EventV2Base, contactOpenID(event), "created"))
		}).
			OnP2UserUpdatedV3(func(ctx context.Context, event *larkcontact.P2UserUpdatedV3) error {
				return contactHandler.HandleContactChange(ctx, toContactChangeEvent(event.EventV2Base, contactOpenID(event), "updated"))
			}).
			OnP2UserDeletedV3(func(ctx context.Context, event *larkcontact.P2UserDeletedV3) error {
				return contactHandler.HandleContactChange(ctx, toContactChangeEvent(event.EventV2Base, contactOpenID(event), "deleted"))
			})
	}
	if meetingRoomHandler != nil {
		eventDispatcher = eventDispatcher.OnP2MeetingRoomStatusChangedV1(func(ctx context.Context, event *larkmeetingroom.P2MeetingRoomStatusChangedV1) error {
			value := MeetingRoomStatusChangeEvent{}
			if event != nil && event.EventV2Base != nil && event.EventV2Base.Header != nil {
				value.EventID = event.EventV2Base.Header.EventID
			}
			if event != nil && event.Event != nil {
				value.RoomID = stringValue(event.Event.RoomId)
				value.Name = stringValue(event.Event.RoomName)
			}
			return meetingRoomHandler.HandleMeetingRoomStatusChange(ctx, value)
		})
	}
	return eventDispatcher
}

func toContactChangeEvent(base *larkevent.EventV2Base, openID, change string) ContactChangeEvent {
	value := ContactChangeEvent{OpenID: openID, Change: change}
	if base != nil && base.Header != nil {
		value.EventID = base.Header.EventID
	}
	return value
}

func contactOpenID(event any) string {
	switch value := event.(type) {
	case *larkcontact.P2UserCreatedV3:
		if value != nil && value.Event != nil && value.Event.Object != nil {
			return stringValue(value.Event.Object.OpenId)
		}
	case *larkcontact.P2UserUpdatedV3:
		if value != nil && value.Event != nil && value.Event.Object != nil {
			return stringValue(value.Event.Object.OpenId)
		}
	case *larkcontact.P2UserDeletedV3:
		if value != nil && value.Event != nil && value.Event.Object != nil {
			return stringValue(value.Event.Object.OpenId)
		}
	}
	return ""
}

func toMessageEvent(event *larkim.P2MessageReceiveV1) MessageEvent {
	result := MessageEvent{}
	if event == nil {
		return result
	}
	if event.EventV2Base != nil && event.EventV2Base.Header != nil {
		result.EventID = event.EventV2Base.Header.EventID
	}
	if event.Event == nil {
		return result
	}
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		result.OpenID = stringValue(event.Event.Sender.SenderId.OpenId)
	}
	if event.Event.Message == nil {
		return result
	}
	message := event.Event.Message
	result.ChatID = stringValue(message.ChatId)
	result.MessageID = stringValue(message.MessageId)
	result.MessageType = stringValue(message.MessageType)
	result.Content = stringValue(message.Content)
	for _, mention := range message.Mentions {
		if mention != nil && mention.Key != nil {
			result.MentionKeys = append(result.MentionKeys, *mention.Key)
		}
	}
	return result
}

func toCardActionEvent(event *callback.CardActionTriggerEvent) CardActionEvent {
	result := CardActionEvent{}
	if event == nil {
		return result
	}
	if event.EventV2Base != nil && event.EventV2Base.Header != nil {
		result.EventID = event.EventV2Base.Header.EventID
	}
	if event.Event == nil {
		return result
	}
	if event.Event.Operator != nil {
		result.OpenID = event.Event.Operator.OpenID
	}
	if event.Event.Context != nil {
		result.ChatID = event.Event.Context.OpenChatID
		result.MessageID = event.Event.Context.OpenMessageID
	}
	if event.Event.Action != nil {
		result.Name = event.Event.Action.Name
		result.Value = event.Event.Action.Value
		result.FormValue = event.Event.Action.FormValue
		result.Options = event.Event.Action.Options
		result.Option = event.Event.Action.Option
		result.InputValue = event.Event.Action.InputValue
	}
	return result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
