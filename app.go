package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type LogEntry struct {
	Time     string `json:"time"`
	Platform string `json:"platform"`
	User     string `json:"user"`
	Text     string `json:"text"`
	Result   string `json:"result"`
	OK       bool   `json:"ok"`
}

type SourceState struct {
	Enabled bool   `json:"enabled"`
	State   string `json:"state"`
	Detail  string `json:"detail"`
}

type SiteState struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
	LastOK  string `json:"lastOk"`
	Error   string `json:"error"`
}

// State é o que o painel e o overlay recebem (via /api/events).
type State struct {
	Version    string      `json:"version"`
	Configured bool        `json:"configured"`
	Queue      QueueView   `json:"queue"`
	Twitch     SourceState `json:"twitch"`
	Kick       SourceState `json:"kick"`
	Songs      int         `json:"songs"`
	SongsErr   string      `json:"songsErr"`
	Log        []LogEntry  `json:"log"`
	Overlay    OverlayCfg  `json:"overlay"`
	Rules      Rules       `json:"rules"`
	Site       SiteState   `json:"site"`
}

type App struct {
	dir   string
	lib   *Library
	queue *Queue
	h     *Handler
	hub   *Hub
	pub   *Publisher

	mu       sync.RWMutex
	cfg      Config
	songsErr string
	log      []LogEntry

	msgs     chan ChatMessage
	twStatus *Status
	kkStatus *Status

	chatMu     sync.Mutex
	chatCancel context.CancelFunc
	sources    map[string]ChatSource
	rootCtx    context.Context
}

func NewApp(dir string) (*App, error) {
	cfg, cfgErr := loadConfig(dir)
	a := &App{
		dir:      dir,
		cfg:      cfg,
		lib:      &Library{},
		queue:    newQueue(dir),
		hub:      newHub(),
		msgs:     make(chan ChatMessage, 256),
		twStatus: &Status{State: "desligado"},
		kkStatus: &Status{State: "desligado"},
		sources:  map[string]ChatSource{},
	}
	a.twStatus.notify = a.broadcast
	a.kkStatus.notify = a.broadcast
	a.h = newHandler(a.lib, a.queue, func() Rules { return a.Config().Rules }, func() string { return a.Config().Site.URL })
	a.pub = newPublisher(a)
	if cfgErr != nil {
		a.songsErr = "config.json inválido, usando padrões: " + cfgErr.Error()
	}
	return a, nil
}

func (a *App) Config() Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

// Run inicia o processamento do chat e as conexões.
func (a *App) Run(ctx context.Context) {
	a.rootCtx = ctx
	go a.pub.Run(ctx)
	go a.consume(ctx)
	if a.Config().Configured {
		a.reloadSongs()
	}
	a.restartChat()
}

func (a *App) reloadSongs() {
	cfg := a.Config()
	songs, err := loadSongs(cfg.Songs)
	a.mu.Lock()
	if err != nil {
		a.songsErr = err.Error()
	} else {
		a.songsErr = ""
	}
	a.mu.Unlock()
	if err == nil {
		a.lib.Set(songs)
		if a.pub.Enabled() && cfg.Site.AutoPublish {
			go func() { a.pub.record(a.pub.PublishSongs(false)) }()
		}
	}
	a.broadcast()
}

func (a *App) restartChat() {
	a.chatMu.Lock()
	defer a.chatMu.Unlock()
	if a.chatCancel != nil {
		a.chatCancel()
	}
	a.sources = map[string]ChatSource{}
	cfg := a.Config()
	ctx, cancel := context.WithCancel(a.rootCtx)
	a.chatCancel = cancel
	if cfg.Twitch.Enabled && cfg.Twitch.Channel != "" {
		t := newTwitchChat(cfg.Twitch, a.msgs, a.twStatus)
		a.sources["twitch"] = t
		go t.Run(ctx)
	} else {
		a.twStatus.Set("desligado", "")
	}
	if cfg.Kick.Enabled && cfg.Kick.Channel != "" {
		k := newKickChat(cfg.Kick, a.msgs, a.kkStatus)
		a.sources["kick"] = k
		go k.Run(ctx)
	} else {
		a.kkStatus.Set("desligado", "")
	}
}

func (a *App) consume(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-a.msgs:
			a.process(m, false)
		}
	}
}

// process trata uma mensagem de chat (real ou simulada pelo painel).
func (a *App) process(m ChatMessage, simulated bool) {
	res, ok := a.h.Handle(m)
	if !ok {
		return
	}
	a.addLog(LogEntry{Time: time.Now().Format("15:04:05"), Platform: m.Platform, User: m.User, Text: m.Text, Result: res.Log, OK: res.OK})
	if res.Reply != "" && !simulated {
		a.chatMu.Lock()
		src := a.sources[m.Platform]
		a.chatMu.Unlock()
		if src != nil {
			src.Send(res.Reply)
		}
	}
	if res.Changed {
		a.changed()
	} else {
		a.broadcast()
	}
}

func (a *App) addLog(e LogEntry) {
	a.mu.Lock()
	a.log = append(a.log, e)
	if len(a.log) > 40 {
		a.log = a.log[len(a.log)-40:]
	}
	a.mu.Unlock()
}

// changed deve ser chamado depois de qualquer mudança na fila.
func (a *App) changed() {
	a.broadcast()
	a.pub.TriggerQueue()
}

func (a *App) State() State {
	cfg := a.Config()
	a.mu.RLock()
	logs := append([]LogEntry{}, a.log...)
	songsErr := a.songsErr
	a.mu.RUnlock()
	ts, td := a.twStatus.Get()
	ks, kd := a.kkStatus.Get()
	ok, perr := a.pub.Info()
	site := SiteState{Enabled: a.pub.Enabled(), URL: cfg.Site.URL, Error: perr}
	if !ok.IsZero() {
		site.LastOK = ok.Format("15:04:05")
	}
	return State{
		Version:    version,
		Configured: cfg.Configured,
		Queue:      a.queue.View(),
		Twitch:     SourceState{cfg.Twitch.Enabled && cfg.Twitch.Channel != "", ts, td},
		Kick:       SourceState{cfg.Kick.Enabled && cfg.Kick.Channel != "", ks, kd},
		Songs:      a.lib.Count(),
		SongsErr:   songsErr,
		Log:        logs,
		Overlay:    cfg.Overlay,
		Rules:      cfg.Rules,
		Site:       site,
	}
}

func (a *App) broadcast() {
	b, err := json.Marshal(a.State())
	if err == nil {
		a.hub.Broadcast(b)
	}
}

// ---------- hub de eventos (SSE) ----------

type Hub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func newHub() *Hub { return &Hub{subs: map[chan []byte]struct{}{}} }

func (h *Hub) Sub() chan []byte {
	ch := make(chan []byte, 4)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsub(ch chan []byte) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

func (h *Hub) Broadcast(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- b:
		default: // cliente lento: pula, o próximo evento traz o estado completo
		}
	}
}
