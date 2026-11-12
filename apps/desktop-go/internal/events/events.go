package events

import "github.com/wailsapp/wails/v3/pkg/application"

const (
	ChatDelta   = "chat:delta"
	ChatDone    = "chat:done"
	ChatError   = "chat:error"
	ChatState   = "chat:state"
	MascotScale = "mascot:scale"
)

type ChatDeltaEvent struct {
	MessageID string `json:"messageId"`
	Text      string `json:"text"`
}

type ChatDoneEvent struct {
	MessageID string `json:"messageId"`
	Text      string `json:"text"`
}

type ChatErrorEvent struct {
	MessageID string `json:"messageId,omitempty"`
	Message   string `json:"message"`
}

type ChatStateEvent struct {
	State string `json:"state"`
}

type MascotScaleEvent struct {
	Scale float64 `json:"scale"`
}

func Register() {
	application.RegisterEvent[ChatDeltaEvent](ChatDelta)
	application.RegisterEvent[ChatDoneEvent](ChatDone)
	application.RegisterEvent[ChatErrorEvent](ChatError)
	application.RegisterEvent[ChatStateEvent](ChatState)
	application.RegisterEvent[MascotScaleEvent](MascotScale)
}
