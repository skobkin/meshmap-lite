package httpapi

type healthStatusPayload struct {
	Status string `json:"status"`
}

type errorPayload struct {
	Error string `json:"error"`
}

type metaPayload struct {
	AppName               string         `json:"app_name"`
	Version               string         `json:"version"`
	WebsocketPath         string         `json:"websocket_path"`
	ChatEnabled           bool           `json:"chat_enabled"`
	DefaultChatChannel    string         `json:"default_chat_channel"`
	ShowRecentMessages    int            `json:"show_recent_messages"`
	LogLiveUpdates        bool           `json:"log_live_updates"`
	LogPageSizeDefault    int            `json:"log_page_size_default"`
	DisconnectedThreshold string         `json:"disconnected_threshold"`
	Map                   metaMapPayload `json:"map"`
}

type metaMapPayload struct {
	Clustering           bool                   `json:"clustering"`
	HidePositionAfter    string                 `json:"hide_position_after"`
	PrecisionCirclesMode string                 `json:"precision_circles_mode"`
	DefaultView          metaDefaultViewPayload `json:"default_view"`
}

type metaDefaultViewPayload struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Zoom      int     `json:"zoom"`
}

type channelPayload struct {
	Name        string `json:"name"`
	ChatEnabled bool   `json:"chat_enabled"`
	IsPrimary   bool   `json:"is_primary"`
}

type heartbeatPayload struct {
	Status string `json:"status"`
}
