package main

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ChatSource é uma conexão com o chat de uma plataforma.
type ChatSource interface {
	Run(ctx context.Context)
	Send(text string)
}

// Status guarda o estado da conexão para mostrar no painel.
type Status struct {
	mu     sync.Mutex
	State  string // "desligado", "conectando", "conectado", "erro"
	Detail string
	notify func()
}

func (s *Status) Set(state, detail string) {
	s.mu.Lock()
	s.State, s.Detail = state, detail
	n := s.notify
	s.mu.Unlock()
	if n != nil {
		n()
	}
}

func (s *Status) Get() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State, s.Detail
}

// ---------- Twitch (IRC sobre TLS) ----------

type TwitchChat struct {
	cfg    TwitchCfg
	out    chan<- ChatMessage
	status *Status
	send   chan string
}

func newTwitchChat(cfg TwitchCfg, out chan<- ChatMessage, st *Status) *TwitchChat {
	return &TwitchChat{cfg: cfg, out: out, status: st, send: make(chan string, 16)}
}

func (t *TwitchChat) Send(text string) {
	select {
	case t.send <- text:
	default:
	}
}

func (t *TwitchChat) canReply() bool {
	return t.cfg.Reply && t.cfg.BotUser != "" && t.cfg.Token != ""
}

func (t *TwitchChat) Run(ctx context.Context) {
	backoff := 2 * time.Second
	for ctx.Err() == nil {
		t.status.Set("conectando", "")
		err := t.session(ctx)
		if ctx.Err() != nil {
			return
		}
		msg := "conexão perdida"
		if err != nil {
			msg = err.Error()
		}
		t.status.Set("erro", msg+" — tentando de novo")
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (t *TwitchChat) session(ctx context.Context) error {
	// WebSocket na porta 443: passa por firewalls e proxies melhor que o IRC puro (6697).
	d := websocket.Dialer{HandshakeTimeout: 15 * time.Second, Proxy: http.ProxyFromEnvironment}
	conn, _, err := d.DialContext(ctx, "wss://irc-ws.chat.twitch.tv:443", nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() { <-ctx.Done(); conn.Close() }()

	nick := fmt.Sprintf("justinfan%d", 10000+rand.Intn(80000))
	pass := "SCHMOOPIIE"
	if t.canReply() {
		nick, pass = t.cfg.BotUser, "oauth:"+t.cfg.Token
	}
	var wmu sync.Mutex
	write := func(s string) {
		wmu.Lock()
		defer wmu.Unlock()
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		conn.WriteMessage(websocket.TextMessage, []byte(s))
	}
	write("PASS " + pass)
	write("NICK " + nick)
	write("CAP REQ :twitch.tv/tags twitch.tv/commands")
	write("JOIN #" + t.cfg.Channel)

	if t.canReply() {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case s := <-t.send:
					write("PRIVMSG #" + t.cfg.Channel + " :" + s)
					time.Sleep(1500 * time.Millisecond) // respeita o limite de mensagens
				}
			}
		}()
	}

	for {
		conn.SetReadDeadline(time.Now().Add(6 * time.Minute))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\r\n") {
			if err := t.handleLine(line, write); err != nil {
				return err
			}
		}
	}
}

func (t *TwitchChat) handleLine(line string, write func(string)) error {
	ir := parseIRC(line)
	switch ir.Cmd {
	case "PING":
		write("PONG :" + strings.TrimPrefix(strings.Join(ir.Params, " "), ":"))
	case "001":
		detail := "somente leitura"
		if t.canReply() {
			detail = "lendo e respondendo como " + t.cfg.BotUser
		}
		t.status.Set("conectado", "#"+t.cfg.Channel+" · "+detail)
	case "NOTICE":
		if strings.Contains(line, "Login authentication failed") || strings.Contains(line, "Improperly formatted auth") {
			return fmt.Errorf("token da Twitch recusado (gere um novo em Configurações)")
		}
	case "RECONNECT":
		return fmt.Errorf("a Twitch pediu reconexão")
	case "PRIVMSG":
		if len(ir.Params) < 2 {
			return nil
		}
		user := ir.Tags["display-name"]
		if user == "" {
			user = strings.SplitN(ir.Prefix, "!", 2)[0]
		}
		text := ir.Params[len(ir.Params)-1]
		select {
		case t.out <- ChatMessage{Platform: "twitch", User: user, Text: text, Roles: twitchRoles(ir.Tags["badges"])}:
		default:
		}
	}
	return nil
}

func twitchRoles(badges string) Roles {
	var r Roles
	for _, b := range strings.Split(badges, ",") {
		name, _, _ := strings.Cut(b, "/")
		switch name {
		case "broadcaster":
			r.Broadcaster = true
		case "moderator":
			r.Mod = true
		case "vip":
			r.VIP = true
		case "subscriber", "founder":
			r.Sub = true
		}
	}
	return r
}

// IRCLine é uma linha de IRC já separada.
type IRCLine struct {
	Tags   map[string]string
	Prefix string
	Cmd    string
	Params []string // o último pode ser o "trailing" (sem os dois pontos)
}

func parseIRC(line string) IRCLine {
	var l IRCLine
	l.Tags = map[string]string{}
	if strings.HasPrefix(line, "@") {
		tagStr, rest, _ := strings.Cut(line[1:], " ")
		for _, kv := range strings.Split(tagStr, ";") {
			k, v, _ := strings.Cut(kv, "=")
			l.Tags[k] = v
		}
		line = rest
	}
	if strings.HasPrefix(line, ":") {
		p, rest, _ := strings.Cut(line[1:], " ")
		l.Prefix, line = p, rest
	}
	var trailing string
	hasTrailing := false
	if i := strings.Index(line, " :"); i >= 0 {
		trailing, hasTrailing = line[i+2:], true
		line = line[:i]
	} else if strings.HasPrefix(line, ":") {
		trailing, hasTrailing, line = line[1:], true, ""
	}
	fields := strings.Fields(line)
	if len(fields) > 0 {
		l.Cmd = fields[0]
		l.Params = fields[1:]
	}
	if hasTrailing {
		l.Params = append(l.Params, trailing)
	}
	return l
}

var _ = net.Dial
