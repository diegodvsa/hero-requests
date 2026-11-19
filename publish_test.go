package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeGitHub simula só o que o publicador usa: branches, refs e contents.
type fakeGitHub struct {
	mu       sync.Mutex
	branch   bool
	files    map[string]string
	shas     map[string]string
	requests []string
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer tok" {
		w.WriteHeader(401)
		return
	}
	p := r.URL.Path
	switch {
	case strings.HasSuffix(p, "/branches/data"):
		if !f.branch {
			w.WriteHeader(404)
			io.WriteString(w, `{"message":"Branch not found"}`)
			return
		}
		io.WriteString(w, `{}`)
	case p == "/repos/u/r":
		io.WriteString(w, `{"default_branch":"main"}`)
	case strings.HasSuffix(p, "/git/ref/heads/main"):
		io.WriteString(w, `{"object":{"sha":"abc"}}`)
	case strings.HasSuffix(p, "/git/refs") && r.Method == "POST":
		f.branch = true
		w.WriteHeader(201)
		io.WriteString(w, `{}`)
	case strings.Contains(p, "/contents/"):
		name := p[strings.LastIndex(p, "/")+1:]
		if r.Method == "GET" {
			if sha, ok := f.shas[name]; ok {
				io.WriteString(w, `{"sha":"`+sha+`"}`)
				return
			}
			w.WriteHeader(404)
			io.WriteString(w, `{}`)
			return
		}
		var b struct{ Content, SHA, Branch string }
		json.NewDecoder(r.Body).Decode(&b)
		if old, ok := f.shas[name]; ok && b.SHA != old {
			w.WriteHeader(409)
			return
		}
		raw, _ := base64.StdEncoding.DecodeString(b.Content)
		f.files[name] = string(raw)
		f.shas[name] = name + "-" + string(rune('a'+len(f.files[name])%26))
		io.WriteString(w, `{}`)
	default:
		w.WriteHeader(404)
	}
}

func TestPublish(t *testing.T) {
	fake := &fakeGitHub{files: map[string]string{}, shas: map[string]string{}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	old := githubAPI
	githubAPI = srv.URL
	defer func() { githubAPI = old }()

	app, _ := NewApp(t.TempDir())
	app.cfg = defaultConfig()
	app.cfg.Site = SiteCfg{Repo: "u/r", Token: "tok", Branch: "data"}
	app.lib.Set(testLib(t).All())
	app.h.queue.Add(app.lib.ByID(0), "ana", "twitch", Roles{}, Rules{MaxPerUser: 2, MaxQueue: 5})

	if err := app.pub.PublishAll(); err != nil {
		t.Fatal(err)
	}
	var songs pubSong
	json.Unmarshal([]byte(fake.files["songs.json"]), &songs)
	var q pubQueue
	json.Unmarshal([]byte(fake.files["queue.json"]), &q)
	if len(songs.Songs) != 7 || len(q.Queue) != 1 || q.Queue[0].User != "ana" || !q.Open {
		t.Fatalf("songs=%d queue=%+v", len(songs.Songs), q)
	}
	// segunda publicação atualiza (usa o sha existente) sem recriar a branch
	if err := app.pub.PublishQueue(); err != nil {
		t.Fatal(err)
	}
	created := 0
	for _, r := range fake.requests {
		if strings.HasSuffix(r, "/git/refs") {
			created++
		}
	}
	if created != 1 {
		t.Errorf("branch criada %d vezes", created)
	}

	app.cfg.Site.Token = "errado"
	if err := app.pub.PublishQueue(); err == nil || !strings.Contains(err.Error(), "token") {
		t.Errorf("token inválido deveria dar erro claro: %v", err)
	}
}
