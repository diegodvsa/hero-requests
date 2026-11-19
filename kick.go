package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// A Kick usa o Pusher para o chat. A chave e o cluster são públicos (vêm do site da Kick)
// e podem ser trocados em Configurações caso a Kick mude.
const (
	defaultPusherCluster = "us2"
	kickUA               = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

// knownPusherKeys são as chaves públicas já usadas pelo site da Kick, da mais nova para a
// mais antiga. Se a conexão for recusada com "app key not found" (código 4001), o app tenta
// a próxima sozinho. Uma chave informada em Configurações tem prioridade e é a única usada.
var knownPusherKeys = []string{"34bf7a0ff419a2a775b9", "eb1d5f283081a78b932c", "32cbd69e4b950bf97679"}

// kickNegotiateURL pergunta à Kick qual chave do Pusher está valendo agora. Assim, quando a
// Kick trocar a chave de novo, o app acompanha sozinho. É variável para os testes.
var kickNegotiateURL = "https://web.kick.com/api/v1/realtime/connection"

type negotiation struct {
	Data struct {
		Connections []struct {
			Provider    string `json:"provider"`
			Credentials struct {
				AppKey  string `json:"app_key"`
				Cluster string `json:"cluster"`
			} `json:"credentials"`
		} `json:"connections"`
	} `json:"data"`
}

// negotiate devolve a chave e o cluster atuais do Pusher segundo a própria Kick.
func negotiate(ctx context.Context) (key, cluster string, err error) {
	body := `{"client":{"id":"hero-requests","type":"web"},"capabilities":{"accepted_providers":[{"provider":"pusher"}]}}`
	req, _ := http.NewRequestWithContext(ctx, "POST", kickNegotiateURL, strings.NewReader(body))
	req.Header.Set("User-Agent", kickUA)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://kick.com")
	req.Header.Set("Referer", "https://kick.com/")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var n negotiation
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&n); err != nil {
		return "", "", err
	}
	for _, c := range n.Data.Connections {
		if c.Provider == "pusher" && c.Credentials.AppKey != "" && c.Credentials.Cluster != "" {
			return c.Credentials.AppKey, c.Credentials.Cluster, nil
		}
	}
	return "", "", errors.New("a Kick não ofereceu o Pusher")
}

// errBadKey indica que o Pusher recusou a chave (códigos 4000 a 4009).
var errBadKey = errors.New("o Pusher recusou a chave do chat da Kick")

func pusherURL(cluster, key string) string {
	if cluster == "" {
		cluster = defaultPusherCluster
	}
	return "wss://ws-" + cluster + ".pusher.com/app/" + key + "?protocol=7&client=js&version=8.4.0&flash=false"
}

type KickChat struct {
	cfg    KickCfg
	out    chan<- ChatMessage
	status *Status
	keyIdx int // posição em knownPusherKeys da chave que está sendo testada

	skipNegotiate bool // a chave negociada foi recusada: usa a lista de chaves conhecidas
}

func (k *KickChat) key() string {
	if k.cfg.PusherKey != "" {
		return k.cfg.PusherKey
	}
	return knownPusherKeys[k.keyIdx%len(knownPusherKeys)]
}

// credentials escolhe a chave e o cluster: o que o usuário informou, depois o que a Kick
// informa agora e, por último, a lista de chaves conhecidas.
func (k *KickChat) credentials(ctx context.Context) (key, cluster string) {
	if k.cfg.PusherKey == "" && !k.skipNegotiate {
		if nk, nc, err := negotiate(ctx); err == nil {
			return nk, nc
		}
	}
	return k.key(), k.cfg.PusherCluster
}

func newKickChat(cfg KickCfg, out chan<- ChatMessage, st *Status) *KickChat {
	return &KickChat{cfg: cfg, out: out, status: st}
}

// Send: responder no chat da Kick exige a API oficial com OAuth; ainda não implementado.
func (k *KickChat) Send(string) {}

func (k *KickChat) Run(ctx context.Context) {
	backoff := 2 * time.Second
	tried := 0
	for ctx.Err() == nil {
		k.status.Set("conectando", "")
		err := k.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errBadKey) && k.cfg.PusherKey == "" {
			if !k.skipNegotiate {
				k.skipNegotiate = true // a chave da Kick falhou; testa a lista conhecida
				continue
			}
			if tried < len(knownPusherKeys)-1 {
				tried++
				k.keyIdx++
				continue // testa a próxima chave conhecida na hora
			}
			k.skipNegotiate = false // deu a volta: tenta negociar de novo na próxima rodada
		}
		tried = 0
		msg := "conexão perdida"
		if err != nil {
			msg = err.Error()
		}
		k.status.Set("erro", msg+" — tentando de novo")
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 60*time.Second {
			backoff *= 2
		}
	}
}

func (k *KickChat) chatroomID(ctx context.Context) (int, error) {
	if k.cfg.ChatroomID > 0 {
		return k.cfg.ChatroomID, nil
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://kick.com/api/v2/channels/"+k.cfg.Channel, nil)
	req.Header.Set("User-Agent", kickUA)
	req.Header.Set("Accept", "application/json")
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 404 {
		return 0, fmt.Errorf("canal %q não existe na Kick", k.cfg.Channel)
	}
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("a Kick bloqueou a consulta automática (HTTP %d). Informe o ID do chat em Configurações", resp.StatusCode)
	}
	var v struct {
		Chatroom struct {
			ID int `json:"id"`
		} `json:"chatroom"`
	}
	if err := json.Unmarshal(body, &v); err != nil || v.Chatroom.ID == 0 {
		return 0, errors.New("resposta inesperada da Kick. Informe o ID do chat em Configurações")
	}
	return v.Chatroom.ID, nil
}

type pusherFrame struct {
	Event   string `json:"event"`
	Channel string `json:"channel"`
	Data    string `json:"data"`
}

func (k *KickChat) session(ctx context.Context) error {
	id, err := k.chatroomID(ctx)
	if err != nil {
		return err
	}
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second, Proxy: http.ProxyFromEnvironment}
	key, cluster := k.credentials(ctx)
	conn, _, err := dialer.DialContext(ctx, pusherURL(cluster, key), http.Header{"User-Agent": {kickUA}})
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() { <-ctx.Done(); conn.Close() }()

	channel := "chatrooms." + strconv.Itoa(id) + ".v2"
	sub := fmt.Sprintf(`{"event":"pusher:subscribe","data":{"auth":"","channel":%q}}`, channel)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(sub)); err != nil {
		return err
	}
	for {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		_, data, err := conn.ReadMessage()
		if err != nil {
			var ce *websocket.CloseError
			if errors.As(err, &ce) && ce.Code >= 4000 && ce.Code <= 4009 {
				return fmt.Errorf("%w (código %d: %s)", errBadKey, ce.Code, ce.Text)
			}
			return err
		}
		var f pusherFrame
		if json.Unmarshal(data, &f) != nil {
			continue
		}
		switch f.Event {
		case "pusher:ping":
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"pusher:pong","data":{}}`))
		case "pusher_internal:subscription_succeeded":
			k.status.Set("conectado", k.cfg.Channel+" · somente leitura")
		case "pusher:error":
			var pe struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal([]byte(f.Data), &pe)
			if pe.Code >= 4000 && pe.Code <= 4009 {
				return fmt.Errorf("%w (código %d: %s)", errBadKey, pe.Code, pe.Message)
			}
			return fmt.Errorf("erro do Pusher: %s", f.Data)
		case `App\Events\ChatMessageEvent`:
			if m, ok := parseKickMessage(f.Data); ok {
				select {
				case k.out <- m:
				default:
				}
			}
		}
	}
}

func parseKickMessage(data string) (ChatMessage, bool) {
	var v struct {
		Content string `json:"content"`
		Sender  struct {
			Username string `json:"username"`
			Identity struct {
				Badges []struct {
					Type string `json:"type"`
				} `json:"badges"`
			} `json:"identity"`
		} `json:"sender"`
	}
	if json.Unmarshal([]byte(data), &v) != nil || v.Content == "" || v.Sender.Username == "" {
		return ChatMessage{}, false
	}
	var r Roles
	for _, b := range v.Sender.Identity.Badges {
		switch strings.ToLower(b.Type) {
		case "broadcaster":
			r.Broadcaster = true
		case "moderator":
			r.Mod = true
		case "vip":
			r.VIP = true
		case "subscriber", "founder", "og":
			r.Sub = true
		}
	}
	return ChatMessage{Platform: "kick", User: v.Sender.Username, Text: v.Content, Roles: r}, true
}
