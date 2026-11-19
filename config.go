package main

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Config é tudo o que o usuário configura na tela inicial / em Configurações.
type Config struct {
	Configured bool       `json:"configured"`
	Port       int        `json:"port"`
	Twitch     TwitchCfg  `json:"twitch"`
	Kick       KickCfg    `json:"kick"`
	Songs      SongsCfg   `json:"songs"`
	Rules      Rules      `json:"rules"`
	Site       SiteCfg    `json:"site"`
	Overlay    OverlayCfg `json:"overlay"`
}

type TwitchCfg struct {
	Enabled bool   `json:"enabled"`
	Channel string `json:"channel"`
	// Para o bot responder no chat (opcional):
	Reply   bool   `json:"reply"`
	BotUser string `json:"botUser"`
	Token   string `json:"token"`
}

type KickCfg struct {
	Enabled bool   `json:"enabled"`
	Channel string `json:"channel"`
	// ChatroomID é opcional: se a Kick bloquear a consulta automática, informe aqui.
	ChatroomID int `json:"chatroomId"`
	// Chave e cluster do Pusher (avançado): só mude se a Kick trocar.
	PusherKey     string `json:"pusherKey"`
	PusherCluster string `json:"pusherCluster"`
}

type SongsCfg struct {
	Mode string `json:"mode"` // "json" ou "folder"
	Path string `json:"path"`
}

type Rules struct {
	MaxPerUser      int  `json:"maxPerUser"`
	MaxQueue        int  `json:"maxQueue"`
	BlockDuplicates bool `json:"blockDuplicates"`
	PriorityRoles   bool `json:"priorityRoles"`
}

type SiteCfg struct {
	Repo        string `json:"repo"`   // "usuario/repositorio"
	Branch      string `json:"branch"` // branch de dados, padrão "data"
	Token       string `json:"token"`
	AutoPublish bool   `json:"autoPublish"`
	URL         string `json:"url"` // opcional, para o comando !lista
}

func defaultConfig() Config {
	return Config{
		Port:    7777,
		Twitch:  TwitchCfg{Enabled: true},
		Kick:    KickCfg{Enabled: true},
		Songs:   SongsCfg{Mode: "json"},
		Rules:   Rules{MaxPerUser: 2, MaxQueue: 50, BlockDuplicates: true, PriorityRoles: false},
		Site:    SiteCfg{Branch: "data", AutoPublish: true},
		Overlay: defaultOverlay(),
	}
}

// ConfigView é o que o navegador recebe: tokens nunca saem do app.
type ConfigView struct {
	Config
	TwitchTokenSet bool `json:"twitchTokenSet"`
	SiteTokenSet   bool `json:"siteTokenSet"`
}

func (c Config) View() ConfigView {
	v := ConfigView{Config: c, TwitchTokenSet: c.Twitch.Token != "", SiteTokenSet: c.Site.Token != ""}
	v.Twitch.Token = ""
	v.Site.Token = ""
	return v
}

const clearToken = "__clear__"

// Merge aplica uma nova configuração mantendo tokens quando vierem vazios.
func (c Config) Merge(n Config) Config {
	switch n.Twitch.Token {
	case "":
		n.Twitch.Token = c.Twitch.Token
	case clearToken:
		n.Twitch.Token = ""
	}
	switch n.Site.Token {
	case "":
		n.Site.Token = c.Site.Token
	case clearToken:
		n.Site.Token = ""
	}
	return n.Normalize()
}

var nonChan = regexp.MustCompile(`[^a-z0-9_]`)

// cleanChannel aceita "nome", "#nome" ou a URL completa do canal.
func cleanChannel(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "/") {
		if u, err := url.Parse(s); err == nil && u.Host != "" {
			s = strings.Trim(u.Path, "/")
		}
		if i := strings.LastIndex(s, "/"); i >= 0 {
			s = s[i+1:]
		}
	}
	s = strings.ToLower(strings.TrimPrefix(s, "#"))
	return nonChan.ReplaceAllString(s, "")
}

func (c Config) Normalize() Config {
	d := defaultConfig()
	if c.Port < 1024 || c.Port > 65535 {
		c.Port = d.Port
	}
	c.Twitch.Channel = cleanChannel(c.Twitch.Channel)
	c.Twitch.BotUser = cleanChannel(c.Twitch.BotUser)
	c.Twitch.Token = strings.TrimPrefix(strings.TrimSpace(c.Twitch.Token), "oauth:")
	c.Kick.Channel = strings.ReplaceAll(cleanChannelKeepDash(c.Kick.Channel), "_", "-")
	c.Kick.PusherKey = strings.TrimSpace(c.Kick.PusherKey)
	c.Kick.PusherCluster = strings.ToLower(strings.TrimSpace(c.Kick.PusherCluster))
	if c.Kick.ChatroomID < 0 {
		c.Kick.ChatroomID = 0
	}
	if c.Songs.Mode != "folder" {
		c.Songs.Mode = "json"
	}
	c.Songs.Path = strings.Trim(strings.TrimSpace(c.Songs.Path), `"`)
	if c.Rules.MaxPerUser < 1 {
		c.Rules.MaxPerUser = d.Rules.MaxPerUser
	}
	if c.Rules.MaxQueue < 1 {
		c.Rules.MaxQueue = d.Rules.MaxQueue
	}
	c.Site.Repo = strings.Trim(strings.TrimSpace(c.Site.Repo), "/")
	c.Site.Repo = strings.TrimPrefix(c.Site.Repo, "https://github.com/")
	c.Site.Token = strings.TrimSpace(c.Site.Token)
	if strings.TrimSpace(c.Site.Branch) == "" {
		c.Site.Branch = d.Site.Branch
	}
	c.Site.URL = strings.TrimSpace(c.Site.URL)
	if c.Site.URL == "" {
		if owner, repo, ok := strings.Cut(c.Site.Repo, "/"); ok && owner != "" && repo != "" {
			c.Site.URL = "https://" + strings.ToLower(owner) + ".github.io/" + repo + "/"
		}
	}
	c.Overlay = c.Overlay.Normalize()
	return c
}

var nonKick = regexp.MustCompile(`[^a-z0-9_-]`)

func cleanChannelKeepDash(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "/") {
		s = strings.Trim(s, "/")
		if i := strings.LastIndex(s, "/"); i >= 0 {
			s = s[i+1:]
		}
	}
	return nonKick.ReplaceAllString(strings.ToLower(s), "")
}

func loadConfig(dir string) (Config, error) {
	c := defaultConfig()
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return defaultConfig(), err
	}
	return c.Normalize(), nil
}

func saveConfig(dir string, c Config) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	// 0600: o arquivo guarda tokens.
	return writeFileAtomic(filepath.Join(dir, "config.json"), b, 0o600)
}

func writeFileAtomic(path string, b []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
