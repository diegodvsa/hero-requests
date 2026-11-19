package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// githubAPI é variável para os testes apontarem para um servidor falso.
var githubAPI = "https://api.github.com"

// Publisher envia a lista de músicas e a fila para uma branch do GitHub.
// O site estático (pasta docs/, no GitHub Pages) lê esses arquivos.
type Publisher struct {
	app     *App
	trigger chan struct{}

	mu         sync.Mutex
	lastOK     time.Time
	lastErr    string
	songsHash  string
	branchDone string
}

func newPublisher(app *App) *Publisher {
	return &Publisher{app: app, trigger: make(chan struct{}, 1)}
}

func (p *Publisher) Enabled() bool {
	c := p.app.Config().Site
	return c.Repo != "" && strings.Contains(c.Repo, "/") && c.Token != ""
}

func (p *Publisher) Info() (ok time.Time, errMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastOK, p.lastErr
}

// TriggerQueue pede uma publicação da fila (com espera para juntar mudanças).
func (p *Publisher) TriggerQueue() {
	if !p.Enabled() || !p.app.Config().Site.AutoPublish {
		return
	}
	select {
	case p.trigger <- struct{}{}:
	default:
	}
}

func (p *Publisher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.trigger:
		}
		// espera 12 s para agrupar várias mudanças em uma só publicação
		select {
		case <-ctx.Done():
			return
		case <-time.After(12 * time.Second):
		}
		select {
		case <-p.trigger:
		default:
		}
		p.record(p.PublishQueue())
	}
}

func (p *Publisher) record(err error) {
	p.mu.Lock()
	if err != nil {
		p.lastErr = err.Error()
	} else {
		p.lastErr = ""
		p.lastOK = time.Now()
	}
	p.mu.Unlock()
	p.app.broadcast()
}

// PublishAll publica músicas e fila (botão "Publicar agora").
func (p *Publisher) PublishAll() error {
	if !p.Enabled() {
		return fmt.Errorf("configure o repositório e o token do GitHub em Configurações")
	}
	err := p.PublishSongs(true)
	if err == nil {
		err = p.PublishQueue()
	}
	p.record(err)
	return err
}

type pubSong struct {
	Updated string     `json:"updated"`
	Songs   [][]string `json:"songs"` // artista, música, álbum, gênero, charter, ano, fonte
}

type pubItem struct {
	Artist   string `json:"artist"`
	Title    string `json:"title"`
	User     string `json:"user"`
	Platform string `json:"platform"`
}

type pubQueue struct {
	Updated string    `json:"updated"`
	Open    bool      `json:"open"`
	Channel string    `json:"channel"`
	Current *pubItem  `json:"current"`
	Queue   []pubItem `json:"queue"`
}

func toPub(it *Item) pubItem {
	return pubItem{Artist: it.Song.Artist, Title: it.Song.Title, User: it.User, Platform: it.Platform}
}

func (p *Publisher) PublishSongs(force bool) error {
	songs := p.app.lib.All()
	if len(songs) == 0 {
		return fmt.Errorf("nenhuma música carregada")
	}
	rows := make([][]string, len(songs))
	for i, s := range songs {
		rows[i] = []string{s.Artist, s.Title, s.Album, s.Genre, s.Charter, s.Year, s.Source}
	}
	b, _ := json.Marshal(pubSong{Updated: time.Now().UTC().Format(time.RFC3339), Songs: rows})
	// o hash ignora a data, para não republicar à toa
	sum := sha256.Sum256([]byte(fmt.Sprint(rows)))
	h := hex.EncodeToString(sum[:])
	p.mu.Lock()
	same := p.songsHash == h
	p.mu.Unlock()
	if same && !force {
		return nil
	}
	if err := p.putFile("songs.json", b, "Atualiza lista de músicas"); err != nil {
		return err
	}
	p.mu.Lock()
	p.songsHash = h
	p.mu.Unlock()
	return nil
}

func (p *Publisher) PublishQueue() error {
	v := p.app.queue.View()
	cfg := p.app.Config()
	ch := cfg.Twitch.Channel
	if ch == "" {
		ch = cfg.Kick.Channel
	}
	pq := pubQueue{Updated: time.Now().UTC().Format(time.RFC3339), Open: v.Open, Channel: ch, Queue: []pubItem{}}
	if v.Current != nil {
		c := toPub(v.Current)
		pq.Current = &c
	}
	for _, it := range v.Items {
		pq.Queue = append(pq.Queue, toPub(it))
	}
	b, _ := json.Marshal(pq)
	return p.putFile("queue.json", b, "Atualiza fila")
}

// ---------- API do GitHub ----------

func (p *Publisher) gh(method, path string, body any) (int, []byte, error) {
	cfg := p.app.Config().Site
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, githubAPI+path, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "hero-requests")
	c := &http.Client{Timeout: 60 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	return resp.StatusCode, data, nil
}

func ghError(status int, data []byte) error {
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &e)
	switch status {
	case 401:
		return fmt.Errorf("GitHub recusou o token (401). Gere um novo token")
	case 403, 404:
		return fmt.Errorf("GitHub negou acesso ao repositório (%d: %s). Confira o nome e as permissões do token (Contents: leitura e escrita)", status, e.Message)
	}
	return fmt.Errorf("GitHub respondeu %d: %s", status, e.Message)
}

func (p *Publisher) ensureBranch() error {
	cfg := p.app.Config().Site
	key := cfg.Repo + "@" + cfg.Branch
	p.mu.Lock()
	done := p.branchDone == key
	p.mu.Unlock()
	if done {
		return nil
	}
	st, data, err := p.gh("GET", "/repos/"+cfg.Repo+"/branches/"+url.PathEscape(cfg.Branch), nil)
	if err != nil {
		return err
	}
	if st == 404 {
		// cria a branch de dados a partir da branch padrão
		st, data, err = p.gh("GET", "/repos/"+cfg.Repo, nil)
		if err != nil {
			return err
		}
		if st != 200 {
			return ghError(st, data)
		}
		var repo struct {
			Default string `json:"default_branch"`
		}
		_ = json.Unmarshal(data, &repo)
		st, data, err = p.gh("GET", "/repos/"+cfg.Repo+"/git/ref/heads/"+url.PathEscape(repo.Default), nil)
		if err != nil {
			return err
		}
		if st != 200 {
			return ghError(st, data)
		}
		var ref struct {
			Object struct {
				SHA string `json:"sha"`
			} `json:"object"`
		}
		_ = json.Unmarshal(data, &ref)
		st, data, err = p.gh("POST", "/repos/"+cfg.Repo+"/git/refs", map[string]string{"ref": "refs/heads/" + cfg.Branch, "sha": ref.Object.SHA})
		if err != nil {
			return err
		}
		if st != 201 && st != 422 { // 422: já existe
			return ghError(st, data)
		}
	} else if st != 200 {
		return ghError(st, data)
	}
	p.mu.Lock()
	p.branchDone = key
	p.mu.Unlock()
	return nil
}

func (p *Publisher) putFile(path string, content []byte, msg string) error {
	if err := p.ensureBranch(); err != nil {
		return err
	}
	cfg := p.app.Config().Site
	api := "/repos/" + cfg.Repo + "/contents/" + path
	for attempt := 0; attempt < 2; attempt++ {
		body := map[string]any{
			"message": msg,
			"content": base64.StdEncoding.EncodeToString(content),
			"branch":  cfg.Branch,
		}
		st, data, err := p.gh("GET", api+"?ref="+url.QueryEscape(cfg.Branch), nil)
		if err != nil {
			return err
		}
		if st == 200 {
			var f struct {
				SHA string `json:"sha"`
			}
			_ = json.Unmarshal(data, &f)
			body["sha"] = f.SHA
		} else if st != 404 {
			return ghError(st, data)
		}
		st, data, err = p.gh("PUT", api, body)
		if err != nil {
			return err
		}
		if st == 200 || st == 201 {
			return nil
		}
		if st == 409 || st == 422 { // sha desatualizado: tenta de novo
			continue
		}
		return ghError(st, data)
	}
	return fmt.Errorf("conflito ao gravar %s no GitHub", path)
}
