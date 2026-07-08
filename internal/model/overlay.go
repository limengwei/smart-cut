package model

type OverlayDisplayMode string

const (
	OverlayModeOverlay    OverlayDisplayMode = "overlay"
	OverlayModeFullscreen OverlayDisplayMode = "fullscreen"
)

type OverlayType string

const (
	OverlayCard OverlayType = "card"
)

type OverlayItem struct {
	ID        string          `json:"id"`
	Type      OverlayType     `json:"type"`
	Mode      OverlayDisplayMode `json:"mode"`
	StartMs   int64           `json:"startMs"`
	EndMs     int64           `json:"endMs"`
	Animation AnimationConfig `json:"animation"`
	Position  PositionConfig  `json:"position"`
	Content   CardContent     `json:"content"`
}

type AnimationConfig struct {
	In       string `json:"in"`
	Out      string `json:"out"`
	Duration int    `json:"duration"`
}

type PositionConfig struct {
	X string `json:"x"`
	Y string `json:"y"`
}

type CardContent struct {
	Title        string   `json:"title"`
	Body         string   `json:"body,omitempty"`
	BulletPoints []string `json:"bulletPoints,omitempty"`
	Icon         string   `json:"icon,omitempty"`
	BigNumber    string   `json:"bigNumber,omitempty"`
	AccentColor  string   `json:"accentColor,omitempty"`
	BgColor      string   `json:"bgColor,omitempty"`
}

type OverlayStyle struct {
	AccentColor    string `json:"accentColor"`
	CardBgColor    string `json:"cardBgColor"`
	CardRadius     int    `json:"cardRadius"`
	FontFamily     string `json:"fontFamily"`
	ShowAnimations bool   `json:"showAnimations"`
}