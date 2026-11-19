package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testLib(t *testing.T) *Library {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "songs.json")
	data := `{"songs":[
	 {"artist":"Foo Fighters","name":"Everlong","album":"The Colour and the Shape","genre":"Rock","charter":"A","year":1997},
	 {"artist":"Metallica","name":"Master of Puppets","genre":"Metal"},
	 {"artist":"Metallica","name":"One","genre":"Metal"},
	 {"artist":"Slipknot","name":"Duality"},
	 {"artist":"Legião Urbana","name":"Tempo Perdido"},
	 {"artist":"Queen","name":"Bohemian Rhapsody"},
	 {"artist":"Panic! At The Disco","name":"Bohemian Rhapsody"},
	 {"artist":"Foo Fighters","name":"Everlong","charter":"duplicado"}
	]}`
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	songs, err := loadSongs(SongsCfg{Mode: "json", Path: p})
	if err != nil {
		t.Fatal(err)
	}
	l := &Library{}
	l.Set(songs)
	return l
}

func TestLoadJSONFormats(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"array":    `[{"Artist":"A","Name":"B"}]`,
		"nested":   `{"entries":[{"metadata":{"artist":"A","title":"B"}}]}`,
		"objmap":   `{"x1":{"artist":"A","song_name":"B"},"x2":{"artist":"A","song_name":"C"}}`,
		"bom":      "\xef\xbb\xbf" + `[{"artist":"A","title":"B"}]`,
		"listkeys": `{"artists":"x","songs":[{"artist":["A","B"],"title":"T"}]}`,
	}
	for name, body := range cases {
		p := filepath.Join(dir, name+".json")
		os.WriteFile(p, []byte(body), 0o644)
		songs, err := loadJSON(p)
		if err != nil || len(songs) == 0 {
			t.Errorf("%s: err=%v songs=%d", name, err, len(songs))
		}
	}
	p := filepath.Join(dir, "bad.json")
	os.WriteFile(p, []byte(`{"foo":1}`), 0o644)
	if _, err := loadJSON(p); err == nil {
		t.Error("esperava erro para JSON sem músicas")
	}
}

func TestScanFolder(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "song.ini"), []byte("\xef\xbb\xbf[song]\nname = Everlong\nartist = Foo Fighters\nyear = , 1997\ncharter = X\n"), 0o644)
	songs, err := loadSongs(SongsCfg{Mode: "folder", Path: dir})
	if err != nil || len(songs) != 1 || songs[0].Title != "Everlong" || songs[0].Year != "1997" {
		t.Fatalf("songs=%v err=%v", songs, err)
	}
}

func TestDedupAndSearch(t *testing.T) {
	l := testLib(t)
	if l.Count() != 7 {
		t.Errorf("esperava 7 músicas após remover duplicata, veio %d", l.Count())
	}
	cases := []struct{ q, want string }{
		{"foo fighters - everlong", "Foo Fighters - Everlong"},
		{"EVERLONG", "Foo Fighters - Everlong"},
		{"everlong foo", "Foo Fighters - Everlong"},
		{"legiao urbana tempo perdido", "Legião Urbana - Tempo Perdido"},
		{"tempo perdido - legiao urbana", "Legião Urbana - Tempo Perdido"},
		{"master puppets", "Metallica - Master of Puppets"},
		{"dual", "Slipknot - Duality"},
	}
	for _, c := range cases {
		res := l.Search(c.q, 5)
		if len(res) == 0 || res[0].Song.Label() != c.want {
			t.Errorf("busca %q: %v", c.q, res)
		}
	}
	if len(l.Search("xyz inexistente", 5)) != 0 {
		t.Error("não deveria achar nada")
	}
}

func newTestHandler(t *testing.T) (*Handler, *Queue) {
	t.Helper()
	q := newQueue(t.TempDir())
	rules := Rules{MaxPerUser: 2, MaxQueue: 4, BlockDuplicates: true}
	h := newHandler(testLib(t), q, func() Rules { return rules }, func() string { return "https://x.github.io/r/" })
	return h, q
}

func say(h *Handler, user, text string) Result {
	r, _ := h.Handle(ChatMessage{Platform: "twitch", User: user, Text: text})
	return r
}

func TestRequestFlow(t *testing.T) {
	h, q := newTestHandler(t)
	if r := say(h, "ana", "!pedir foo fighters - everlong"); !r.OK || !strings.Contains(r.Reply, "posição 1") {
		t.Fatalf("pedido simples: %+v", r)
	}
	if r := say(h, "bia", "!pedir everlong"); r.OK || !strings.Contains(r.Reply, "já está na fila") {
		t.Errorf("duplicata: %+v", r)
	}
	// ambíguo: duas "Bohemian Rhapsody"
	r := say(h, "bia", "!pedir bohemian rhapsody")
	if r.OK || !strings.Contains(r.Reply, "1)") || !strings.Contains(r.Reply, "2)") {
		t.Fatalf("ambíguo: %+v", r)
	}
	if r := say(h, "bia", "!pedir 2"); !r.OK {
		t.Errorf("escolha: %+v", r)
	}
	if r := say(h, "bia", "!pedir 1"); r.OK {
		t.Errorf("escolha sem pendência deveria falhar: %+v", r)
	}
	if r := say(h, "bia", "!pedir duality"); !r.OK {
		t.Errorf("segundo pedido: %+v", r)
	}
	if r := say(h, "bia", "!pedir one"); r.OK || !strings.Contains(r.Reply, "pedido(s) na fila") {
		t.Errorf("limite por pessoa: %+v", r)
	}
	if r := say(h, "caio", "!pedir tempo perdido"); !r.OK {
		t.Errorf("quarto pedido: %+v", r)
	}
	if r := say(h, "dani", "!pedir master of puppets"); r.OK || !strings.Contains(r.Reply, "cheia") {
		t.Errorf("fila cheia: %+v", r)
	}
	if r := say(h, "ana", "!sair"); !r.OK || len(q.View().Items) != 3 {
		t.Errorf("sair: %+v", r)
	}
	if r := say(h, "ana", "!pedir nada disso existe"); !strings.Contains(r.Reply, "https://x.github.io/r/") {
		t.Errorf("não encontrada deve citar o site: %+v", r)
	}
	if !strings.Contains(say(h, "x", "!fila").Reply, "Na fila:") || !strings.Contains(say(h, "x", "!lista").Reply, "github.io") {
		t.Error("!fila / !lista")
	}
	if _, handled := h.Handle(ChatMessage{Platform: "twitch", User: "x", Text: "oi pessoal"}); handled {
		t.Error("texto comum não é comando")
	}
}

func TestQueueRulesAndControl(t *testing.T) {
	h, q := newTestHandler(t)
	songs := h.lib.All()
	rules := Rules{MaxPerUser: 5, MaxQueue: 10, PriorityRoles: true}

	q.Add(songs[0], "a", "twitch", Roles{}, rules)
	q.Add(songs[1], "b", "twitch", Roles{}, rules)
	it, pos, _ := q.Add(songs[2], "vip", "kick", Roles{VIP: true}, rules)
	if pos != 1 || !it.Priority {
		t.Errorf("VIP deveria ir para a posição 1, foi %d", pos)
	}
	_, pos, _ = q.Add(songs[3], "sub", "twitch", Roles{Sub: true}, rules)
	if pos != 2 {
		t.Errorf("sub entra depois do VIP, foi %d", pos)
	}

	q.SetOpen(false)
	if _, _, err := q.Add(songs[4], "z", "twitch", Roles{}, rules); err != ErrClosed {
		t.Errorf("fechada: %v", err)
	}
	if _, _, err := q.Add(songs[4], "dono", "twitch", Roles{Broadcaster: true}, rules); err != nil {
		t.Errorf("streamer ignora fechamento: %v", err)
	}

	first := q.View().Items[0].ID
	q.Next()
	v := q.View()
	if v.Current == nil || v.Current.ID != first {
		t.Fatal("Next deveria chamar o primeiro")
	}
	q.Next()
	if len(q.View().History) != 1 {
		t.Error("histórico")
	}
	id := q.View().Items[1].ID
	if !q.Move(id, -1) || q.View().Items[0].ID != id {
		t.Error("mover")
	}
	if !q.Play(q.View().Items[2].ID) || q.View().Current == nil {
		t.Error("play")
	}

	// persistência
	q2 := newQueue(filepath.Dir(q.path))
	if len(q2.View().Items) != len(q.View().Items) || q2.View().Open {
		t.Error("a fila deveria ser restaurada do disco")
	}
	q.Clear()
	if len(q.View().Items) != 0 {
		t.Error("limpar")
	}
}

func TestParseIRC(t *testing.T) {
	l := parseIRC(`@badge-info=;badges=moderator/1,subscriber/12;display-name=Ana :ana!ana@ana.tmi.twitch.tv PRIVMSG #canal :!pedir everlong : teste`)
	if l.Cmd != "PRIVMSG" || l.Tags["display-name"] != "Ana" || l.Params[len(l.Params)-1] != "!pedir everlong : teste" {
		t.Fatalf("%+v", l)
	}
	r := twitchRoles(l.Tags["badges"])
	if !r.Mod || !r.Sub || r.VIP || r.Broadcaster {
		t.Errorf("roles: %+v", r)
	}
	p := parseIRC("PING :tmi.twitch.tv")
	if p.Cmd != "PING" || p.Params[0] != "tmi.twitch.tv" {
		t.Errorf("%+v", p)
	}
	if parseIRC(":tmi.twitch.tv 001 justinfan1 :Welcome, GLHF!").Cmd != "001" {
		t.Error("001")
	}
}

func TestKickMessage(t *testing.T) {
	m, ok := parseKickMessage(`{"id":"1","content":"!pedir duality","type":"message","sender":{"id":1,"username":"Dudu","identity":{"badges":[{"type":"subscriber","text":"Subscriber"},{"type":"vip"}]}}}`)
	if !ok || m.User != "Dudu" || m.Text != "!pedir duality" || !m.Roles.Sub || !m.Roles.VIP {
		t.Fatalf("%+v ok=%v", m, ok)
	}
	if _, ok := parseKickMessage(`{"content":""}`); ok {
		t.Error("mensagem vazia")
	}
}

func TestConfigNormalize(t *testing.T) {
	c := defaultConfig()
	c.Twitch.Channel = " https://www.twitch.tv/MeuCanal/ "
	c.Kick.Channel = "https://kick.com/meu_canal"
	c.Twitch.Token = "oauth:abc"
	c.Site.Repo = "https://github.com/Fulano/hero-requests/"
	c.Port = 80
	n := c.Normalize()
	if n.Twitch.Channel != "meucanal" || n.Kick.Channel != "meu-canal" || n.Twitch.Token != "abc" || n.Port != 7777 {
		t.Errorf("%+v", n)
	}
	if n.Site.Repo != "Fulano/hero-requests" || n.Site.URL != "https://fulano.github.io/hero-requests/" {
		t.Errorf("site: %+v", n.Site)
	}
	// tokens: vazio mantém, __clear__ apaga
	old := n
	upd := n
	upd.Twitch.Token, upd.Site.Token = "", clearToken
	old.Site.Token = "ghp"
	m := old.Merge(upd)
	if m.Twitch.Token != "abc" || m.Site.Token != "" {
		t.Errorf("merge: %+v", m)
	}
	if v := old.View(); v.Twitch.Token != "" || v.Site.Token != "" || !v.SiteTokenSet {
		t.Error("view não pode vazar tokens")
	}
}

func TestSourceField(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(p, []byte(`[{"artist":"A","name":"B","source":"rb3"},{"artist":"C","name":"D","metadata":{"Source":"customs"}}]`), 0o644)
	songs, err := loadJSON(p)
	if err != nil || len(songs) != 2 || songs[0].Source != "rb3" || songs[1].Source != "customs" {
		t.Fatalf("%+v %v", songs, err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "song.ini"), []byte("[song]\nname=X\nartist=Y\nsource = gh3\n"), 0o644)
	ini, _ := scanFolder(dir)
	if len(ini) != 1 || ini[0].Source != "gh3" {
		t.Errorf("song.ini: %+v", ini)
	}
}

func TestOverlayNormalize(t *testing.T) {
	d := defaultOverlay()
	if d != d.Normalize() {
		t.Error("o padrão deve ser estável")
	}
	o := d
	o.Items, o.Width, o.FontSize = 99, 5, 0
	o.Accent, o.BgColor = "vermelho", "#12345"
	o.BgOpacity, o.Blur, o.BorderWidth = 0, 0, 0 // zero é válido nesses campos
	o.Corner = "meio"
	o.FontFamily = "Arial; } body{display:none"
	o.CustomCSS = strings.Repeat("a", 30000)
	n := o.Normalize()
	if n.Items != 12 || n.Width != d.Width || n.FontSize != d.FontSize || n.Accent != d.Accent || n.BgColor != d.BgColor || n.Corner != "tr" {
		t.Errorf("limites: %+v", n)
	}
	if n.BgOpacity != 0 || n.Blur != 0 || n.BorderWidth != 0 {
		t.Errorf("zero deve ser aceito: %+v", n)
	}
	if n.FontFamily != d.FontFamily || len(n.CustomCSS) != 20000 {
		t.Errorf("fonte/css: %q %d", n.FontFamily, len(n.CustomCSS))
	}
	g := d
	g.FontFamily, g.FontGoogle = "Press Start 2P", true
	if g.Normalize().FontFamily != "Press Start 2P" {
		t.Error("nome de fonte do Google deve passar")
	}
}

func TestPusherURL(t *testing.T) {
	k := &KickChat{}
	if u := pusherURL("", k.key()); !strings.Contains(u, "ws-us2.pusher.com/app/"+knownPusherKeys[0]) {
		t.Error(u)
	}
	k.keyIdx = 1
	if k.key() != knownPusherKeys[1] {
		t.Error("deveria usar a segunda chave")
	}
	custom := &KickChat{cfg: KickCfg{PusherKey: "abc", PusherCluster: "eu"}}
	key, cl := custom.credentials(context.Background())
	if u := pusherURL(cl, key); !strings.Contains(u, "ws-eu.pusher.com/app/abc") {
		t.Error(u)
	}
}

func TestKickNegotiate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || !strings.Contains(string(b), `"provider":"pusher"`) {
			w.WriteHeader(400)
			return
		}
		io.WriteString(w, `{"data":{"connections":[{"provider":"centrifugo","credentials":{}},{"provider":"pusher","credentials":{"app_key":"NOVA123","cluster":"eu"}}],"mode":"websocket"},"message":"success"}`)
	}))
	defer srv.Close()
	old := kickNegotiateURL
	kickNegotiateURL = srv.URL
	defer func() { kickNegotiateURL = old }()

	k := &KickChat{}
	key, cl := k.credentials(context.Background())
	if key != "NOVA123" || cl != "eu" {
		t.Errorf("negociada: %s %s", key, cl)
	}
	// se a negociação falha, usa a lista conhecida
	kickNegotiateURL = "http://127.0.0.1:1/nada"
	key, cl = (&KickChat{}).credentials(context.Background())
	if key != knownPusherKeys[0] || cl != "" {
		t.Errorf("reserva: %s %q", key, cl)
	}
	// chave recusada pela Kick: pula a negociação
	k.skipNegotiate = true
	kickNegotiateURL = srv.URL
	if key, _ := k.credentials(context.Background()); key != knownPusherKeys[0] {
		t.Errorf("skipNegotiate: %s", key)
	}
}
