package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ChatMessage struct {
	Platform string // "twitch" ou "kick"
	User     string
	Text     string
	Roles    Roles
}

// Result é o que o comando produziu: a resposta pública e o resumo para o log.
type Result struct {
	Reply   string
	Log     string
	OK      bool
	Changed bool // a fila mudou
}

type pending struct {
	opts []*Song
	exp  time.Time
}

// Handler interpreta os comandos do chat (!pedir, !fila, ...).
type Handler struct {
	lib     *Library
	queue   *Queue
	rules   func() Rules
	siteURL func() string

	mu      sync.Mutex
	pending map[string]pending
}

func newHandler(lib *Library, q *Queue, rules func() Rules, siteURL func() string) *Handler {
	return &Handler{lib: lib, queue: q, rules: rules, siteURL: siteURL, pending: map[string]pending{}}
}

// Handle devolve ok=false quando a mensagem não é um comando.
func (h *Handler) Handle(m ChatMessage) (Result, bool) {
	text := strings.TrimSpace(m.Text)
	if !strings.HasPrefix(text, "!") || len(text) < 2 {
		return Result{}, false
	}
	cmd, args, _ := strings.Cut(text[1:], " ")
	cmd, args = strings.ToLower(cmd), strings.TrimSpace(args)
	switch cmd {
	case "pedir", "sr", "request", "song":
		return h.request(m, args), true
	case "fila", "queue":
		return h.showQueue(m), true
	case "atual", "now", "np":
		return h.showCurrent(m), true
	case "lista", "list", "songs", "musicas":
		return h.showList(m), true
	case "sair", "cancelar", "remover":
		return h.cancel(m), true
	}
	return Result{}, false
}

func at(user string) string { return "@" + user + ", " }

func (h *Handler) request(m ChatMessage, args string) Result {
	if args == "" {
		return Result{Reply: at(m.User) + "use !pedir artista - música", Log: "faltou o nome da música"}
	}
	key := m.Platform + ":" + strings.ToLower(m.User)
	if n, err := strconv.Atoi(args); err == nil {
		h.mu.Lock()
		p, ok := h.pending[key]
		if ok && time.Now().Before(p.exp) && n >= 1 && n <= len(p.opts) {
			delete(h.pending, key)
			h.mu.Unlock()
			return h.enqueue(m, p.opts[n-1])
		}
		h.mu.Unlock()
		return Result{Reply: at(m.User) + "não há escolha pendente. Use !pedir artista - música", Log: "número sem lista pendente"}
	}
	res := h.lib.Search(args, 5)
	switch {
	case len(res) == 0:
		msg := at(m.User) + "não encontrei essa música."
		if u := h.siteURL(); u != "" {
			msg += " Veja a lista: " + u
		}
		return Result{Reply: msg, Log: "não encontrada: " + args}
	case len(res) == 1 || (res[0].Score >= 200 && res[0].Score > res[1].Score):
		return h.enqueue(m, res[0].Song)
	}
	top := res
	if len(top) > 3 {
		top = top[:3]
	}
	opts := make([]*Song, len(top))
	parts := make([]string, len(top))
	for i, r := range top {
		opts[i] = r.Song
		parts[i] = fmt.Sprintf("%d) %s", i+1, r.Song.Label())
	}
	h.mu.Lock()
	h.pending[key] = pending{opts: opts, exp: time.Now().Add(90 * time.Second)}
	h.mu.Unlock()
	return Result{
		Reply: at(m.User) + "achei várias: " + strings.Join(parts, " | ") + " — responda !pedir 1, !pedir 2...",
		Log:   fmt.Sprintf("%d opções, aguardando escolha", len(top)),
	}
}

func (h *Handler) enqueue(m ChatMessage, s *Song) Result {
	it, pos, err := h.queue.Add(s, m.User, m.Platform, m.Roles, h.rules())
	switch err {
	case nil:
		_ = it
		return Result{
			Reply:   fmt.Sprintf("%s\"%s\" entrou na fila (posição %d).", at(m.User), s.Label(), pos),
			Log:     "entrou na fila: " + s.Label(),
			OK:      true,
			Changed: true,
		}
	case ErrClosed:
		return Result{Reply: at(m.User) + "os pedidos estão fechados no momento.", Log: "pedidos fechados"}
	case ErrFull:
		return Result{Reply: at(m.User) + "a fila está cheia, tente mais tarde.", Log: "fila cheia"}
	case ErrUserLimit:
		return Result{Reply: fmt.Sprintf("%svocê já tem %d pedido(s) na fila. Aguarde tocar.", at(m.User), h.rules().MaxPerUser), Log: "limite por pessoa"}
	case ErrDuplicate:
		return Result{Reply: at(m.User) + "essa música já está na fila.", Log: "música repetida"}
	}
	return Result{Log: err.Error()}
}

func (h *Handler) showQueue(m ChatMessage) Result {
	v := h.queue.View()
	if len(v.Items) == 0 {
		return Result{Reply: "A fila está vazia. Peça com !pedir artista - música", Log: "fila vazia", OK: true}
	}
	var parts []string
	for i, it := range v.Items {
		if i == 3 {
			break
		}
		parts = append(parts, fmt.Sprintf("%d) %s (%s)", i+1, it.Song.Label(), it.User))
	}
	msg := "Na fila: " + strings.Join(parts, " | ")
	if len(v.Items) > 3 {
		msg += fmt.Sprintf(" (+%d)", len(v.Items)-3)
	}
	return Result{Reply: msg, Log: "mostrou a fila", OK: true}
}

func (h *Handler) showCurrent(m ChatMessage) Result {
	v := h.queue.View()
	if v.Current == nil {
		return Result{Reply: "Nenhuma música do chat tocando agora.", Log: "sem música atual", OK: true}
	}
	return Result{Reply: fmt.Sprintf("Tocando: %s (pedido de %s)", v.Current.Song.Label(), v.Current.User), Log: "mostrou a atual", OK: true}
}

func (h *Handler) showList(m ChatMessage) Result {
	if u := h.siteURL(); u != "" {
		return Result{Reply: "Lista de músicas: " + u + " — peça com !pedir artista - música", Log: "enviou o link da lista", OK: true}
	}
	return Result{Reply: "Peça com !pedir artista - música", Log: "sem site configurado", OK: true}
}

func (h *Handler) cancel(m ChatMessage) Result {
	if it := h.queue.RemoveLastBy(m.User); it != nil {
		return Result{Reply: fmt.Sprintf("%sseu pedido \"%s\" foi removido.", at(m.User), it.Song.Label()), Log: "removeu o último pedido", OK: true, Changed: true}
	}
	return Result{Reply: at(m.User) + "você não tem pedidos na fila.", Log: "nada para remover"}
}
