package main

import (
	"regexp"
	"strings"
)

// OverlayCfg é a aparência completa do overlay do OBS.
type OverlayCfg struct {
	// Posição e tamanho
	Items  int    `json:"items"`
	Corner string `json:"corner"` // tl, tr, bl, br
	Margin int    `json:"margin"`
	Width  int    `json:"width"`

	// Fonte
	FontFamily string `json:"fontFamily"`
	FontGoogle bool   `json:"fontGoogle"` // carrega a fonte do Google Fonts
	FontSize   int    `json:"fontSize"`
	TitleSize  int    `json:"titleSize"`

	// Cores
	Accent      string `json:"accent"`
	TextColor   string `json:"textColor"`
	MutedColor  string `json:"mutedColor"`
	BgColor     string `json:"bgColor"`
	BorderColor string `json:"borderColor"`

	// Caixas
	BgOpacity     int  `json:"bgOpacity"` // 0 a 100
	BorderOpacity int  `json:"borderOpacity"`
	BorderWidth   int  `json:"borderWidth"`
	Radius        int  `json:"radius"`
	Padding       int  `json:"padding"`
	Gap           int  `json:"gap"`
	Blur          int  `json:"blur"`
	AccentBar     bool `json:"accentBar"`

	// Conteúdo
	ShowNow       bool   `json:"showNow"`
	ShowQueue     bool   `json:"showQueue"`
	ShowHint      bool   `json:"showHint"`
	ShowBadges    bool   `json:"showBadges"`
	ShowRequester bool   `json:"showRequester"`
	ShowLabels    bool   `json:"showLabels"`
	NowLabel      string `json:"nowLabel"`
	QueueLabel    string `json:"queueLabel"`
	HintText      string `json:"hintText"`

	// Avançado
	CustomCSS string `json:"customCss"`
}

func defaultOverlay() OverlayCfg {
	return OverlayCfg{
		Items: 5, Corner: "tr", Margin: 16, Width: 380,
		FontFamily: `"Segoe UI", system-ui, sans-serif`, FontSize: 15, TitleSize: 20,
		Accent: "#ff8a3d", TextColor: "#e8eaf2", MutedColor: "#8b91a5", BgColor: "#0f1117", BorderColor: "#2a2e3c",
		BgOpacity: 100, BorderOpacity: 100, BorderWidth: 1, Radius: 10, Padding: 12, Gap: 8, Blur: 0, AccentBar: true,
		ShowNow: true, ShowQueue: true, ShowHint: true, ShowBadges: true, ShowRequester: true, ShowLabels: true,
		NowLabel: "Tocando agora", QueueLabel: "Próximas", HintText: "Peça com !pedir artista - música",
	}
}

var (
	hexRe  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	fontRe = regexp.MustCompile(`^[\p{L}0-9 ,'"\-]{1,120}$`)
)

// clamp limita v ao intervalo; se zero não for válido (def > 0 e v < min), usa o padrão.
func clamp(v, min, max, def int) int {
	if v < min {
		return def
	}
	if v > max {
		return max
	}
	return v
}

func clean(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > max {
		r = r[:max]
	}
	return string(r)
}

// Normalize corrige valores fora do limite e troca inválidos pelo padrão.
func (o OverlayCfg) Normalize() OverlayCfg {
	d := defaultOverlay()
	o.Items = clamp(o.Items, 1, 12, d.Items)
	switch o.Corner {
	case "tl", "tr", "bl", "br":
	default:
		o.Corner = d.Corner
	}
	o.Margin = clamp(o.Margin, 0, 400, d.Margin)
	o.Width = clamp(o.Width, 200, 1200, d.Width)
	o.FontFamily = strings.TrimSpace(o.FontFamily)
	if !fontRe.MatchString(o.FontFamily) {
		o.FontFamily, o.FontGoogle = d.FontFamily, false
	}
	o.FontSize = clamp(o.FontSize, 8, 60, d.FontSize)
	o.TitleSize = clamp(o.TitleSize, 8, 90, d.TitleSize)
	for _, p := range []struct {
		v   *string
		def string
	}{
		{&o.Accent, d.Accent}, {&o.TextColor, d.TextColor}, {&o.MutedColor, d.MutedColor},
		{&o.BgColor, d.BgColor}, {&o.BorderColor, d.BorderColor},
	} {
		if !hexRe.MatchString(*p.v) {
			*p.v = p.def
		}
	}
	o.BgOpacity = clamp(o.BgOpacity, 0, 100, d.BgOpacity)
	o.BorderOpacity = clamp(o.BorderOpacity, 0, 100, d.BorderOpacity)
	o.BorderWidth = clamp(o.BorderWidth, 0, 12, d.BorderWidth)
	o.Radius = clamp(o.Radius, 0, 60, d.Radius)
	o.Padding = clamp(o.Padding, 0, 60, d.Padding)
	o.Gap = clamp(o.Gap, 0, 60, d.Gap)
	o.Blur = clamp(o.Blur, 0, 40, d.Blur)
	o.NowLabel = clean(o.NowLabel, 40)
	o.QueueLabel = clean(o.QueueLabel, 40)
	o.HintText = clean(o.HintText, 120)
	o.CustomCSS = clean(o.CustomCSS, 20000)
	return o
}
