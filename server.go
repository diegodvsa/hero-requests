package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed web
var webFS embed.FS

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(webFS, "web")
	files := http.FileServerFS(sub)

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { serveFile(w, r, sub, "index.html") })
	mux.HandleFunc("GET /overlay", func(w http.ResponseWriter, r *http.Request) { serveFile(w, r, sub, "overlay.html") })
	mux.Handle("GET /static/", http.StripPrefix("/static/", noCache(files)))

	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, a.State()) })
	mux.HandleFunc("GET /api/events", a.handleEvents)

	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, a.Config().View()) })
	mux.HandleFunc("PUT /api/config", a.handleConfig)

	mux.HandleFunc("GET /api/songs/search", func(w http.ResponseWriter, r *http.Request) {
		res := a.lib.Search(r.URL.Query().Get("q"), 20)
		out := make([]map[string]any, len(res))
		for i, s := range res {
			out[i] = map[string]any{"id": s.Song.ID, "label": s.Song.Label()}
		}
		writeJSON(w, 200, out)
	})
	mux.HandleFunc("POST /api/songs/test", func(w http.ResponseWriter, r *http.Request) {
		var c SongsCfg
		if !readJSON(w, r, &c) {
			return
		}
		songs, err := loadSongs(Config{Songs: c}.Normalize().Songs)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		sample := []string{}
		for i := 0; i < len(songs) && i < 3; i++ {
			sample = append(sample, songs[i].Label())
		}
		writeJSON(w, 200, map[string]any{"count": len(songs), "sample": sample})
	})
	mux.HandleFunc("POST /api/songs/reload", func(w http.ResponseWriter, r *http.Request) {
		a.reloadSongs()
		writeJSON(w, 200, a.State())
	})

	act := func(path string, fn func(r *http.Request, body map[string]any) bool) {
		mux.HandleFunc("POST "+path, func(w http.ResponseWriter, r *http.Request) {
			body := map[string]any{}
			if r.ContentLength != 0 && !readJSON(w, r, &body) {
				return
			}
			if fn(r, body) {
				a.changed()
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
		})
	}
	id := func(b map[string]any) int64 { f, _ := b["id"].(float64); return int64(f) }
	act("/api/queue/next", func(r *http.Request, b map[string]any) bool { a.queue.Next(); return true })
	act("/api/queue/done", func(r *http.Request, b map[string]any) bool { a.queue.Done(); return true })
	act("/api/queue/play", func(r *http.Request, b map[string]any) bool { return a.queue.Play(id(b)) })
	act("/api/queue/remove", func(r *http.Request, b map[string]any) bool { return a.queue.Remove(id(b)) })
	act("/api/queue/clear", func(r *http.Request, b map[string]any) bool { a.queue.Clear(); return true })
	act("/api/queue/move", func(r *http.Request, b map[string]any) bool {
		d, _ := b["dir"].(float64)
		return a.queue.Move(id(b), int(d))
	})
	act("/api/open", func(r *http.Request, b map[string]any) bool {
		o, _ := b["open"].(bool)
		a.queue.SetOpen(o)
		return true
	})
	act("/api/queue/add", func(r *http.Request, b map[string]any) bool {
		sid, _ := b["songId"].(float64)
		s := a.lib.ByID(int(sid))
		if s == nil {
			return false
		}
		user, _ := b["user"].(string)
		if strings.TrimSpace(user) == "" {
			user = "streamer"
		}
		_, _, err := a.queue.Add(s, user, "manual", Roles{}, a.Config().Rules)
		return err == nil
	})
	// simula uma mensagem de chat, útil para testar sem estar ao vivo
	mux.HandleFunc("POST /api/simulate", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			User, Text, Platform string
			Sub                  bool
		}
		if !readJSON(w, r, &b) {
			return
		}
		if b.User == "" {
			b.User = "viewer_teste"
		}
		if b.Platform != "kick" {
			b.Platform = "twitch"
		}
		a.process(ChatMessage{Platform: b.Platform, User: b.User, Text: b.Text, Roles: Roles{Sub: b.Sub}}, true)
		writeJSON(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /api/publish", func(w http.ResponseWriter, r *http.Request) {
		if err := a.pub.PublishAll(); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	})

	return guard(mux)
}

func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

// guard bloqueia acessos que não vêm do próprio computador: impede que um
// site aberto no navegador mande comandos ao app (DNS rebinding / CSRF).
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "[::1]" && host != "::1" {
			http.Error(w, "acesso negado", http.StatusForbidden)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
				http.Error(w, "origem negada", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func (a *App) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming não suportado", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	ch := a.hub.Sub()
	defer a.hub.Unsub(ch)
	first, _ := json.Marshal(a.State())
	fmt.Fprintf(w, "data: %s\n\n", first)
	fl.Flush()
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		}
	}
}

func (a *App) handleConfig(w http.ResponseWriter, r *http.Request) {
	var n Config
	if !readJSON(w, r, &n) {
		return
	}
	old := a.Config()
	merged := old.Merge(n)
	if err := saveConfig(a.dir, merged); err != nil {
		writeJSON(w, 500, map[string]string{"error": "não consegui salvar: " + err.Error()})
		return
	}
	a.mu.Lock()
	a.cfg = merged
	a.mu.Unlock()

	if merged.Songs != old.Songs || a.lib.Count() == 0 {
		a.reloadSongs()
	}
	if merged.Twitch != old.Twitch || merged.Kick != old.Kick || !old.Configured {
		a.restartChat()
	}
	a.broadcast()
	if merged.Site.Repo != old.Site.Repo || merged.Site.Token != old.Site.Token || merged.Site.Branch != old.Site.Branch {
		a.pub.TriggerQueue()
	}
	writeJSON(w, 200, map[string]any{
		"config":         merged.View(),
		"restartForPort": merged.Port != old.Port,
	})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, 400, map[string]string{"error": "JSON inválido"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

var _ = strconv.Itoa
