package events

import "github.com/wailsapp/wails/v3/pkg/application"

const (
	ChatDelta     = "chat:delta"
	ChatDone      = "chat:done"
	ChatError     = "chat:error"
	ChatState     = "chat:state"
	MascotScale   = "mascot:scale"
	ThemeChanged  = "theme:changed"
	BubbleShow    = "bubble:show"
	NavigateRoute = "navigate:route"
)

type ChatDeltaEvent struct {
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
	Text           string `json:"text"`
}

type ChatDoneEvent struct {
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
	Text           string `json:"text"`
}

type ChatErrorEvent struct {
	MessageID      string `json:"messageId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	Message        string `json:"message"`
}

type ChatStateEvent struct {
	State string `json:"state"`
}

type MascotScaleEvent struct {
	Scale float64 `json:"scale"`
}

type ThemeChangedEvent struct {
	Theme string `json:"theme"`
}

type BubbleShowEvent struct {
	Text string `json:"text"`
	Side string `json:"side"`
}

type NavigateRouteEvent struct {
	Route string `json:"route"`
}

func Register() {
	application.RegisterEvent[ChatDeltaEvent](ChatDelta)
	application.RegisterEvent[ChatDoneEvent](ChatDone)
	application.RegisterEvent[ChatErrorEvent](ChatError)
	application.RegisterEvent[ChatStateEvent](ChatState)
	application.RegisterEvent[MascotScaleEvent](MascotScale)
	application.RegisterEvent[ThemeChangedEvent](ThemeChanged)
	application.RegisterEvent[BubbleShowEvent](BubbleShow)
	application.RegisterEvent[NavigateRouteEvent](NavigateRoute)
}
