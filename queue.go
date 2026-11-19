package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Roles struct {
	Broadcaster bool `json:"broadcaster,omitempty"`
	Mod         bool `json:"mod,omitempty"`
	VIP         bool `json:"vip,omitempty"`
	Sub         bool `json:"sub,omitempty"`
}

func (r Roles) Priority() bool { return r.Broadcaster || r.Mod || r.VIP || r.Sub }

// Item é um pedido na fila.
type Item struct {
	ID       int64     `json:"id"`
	Song     *Song     `json:"song"`
	User     string    `json:"user"`
	Platform string    `json:"platform"` // "twitch", "kick" ou "manual"
	Priority bool      `json:"priority,omitempty"`
	At       time.Time `json:"at"`
}

var (
	ErrClosed    = errors.New("fechado")
	ErrFull      = errors.New("fila cheia")
	ErrUserLimit = errors.New("limite por pessoa")
	ErrDuplicate = errors.New("música repetida")
)

type persisted struct {
	Open    bool    `json:"open"`
	NextID  int64   `json:"nextId"`
	Current *Item   `json:"current"`
	Items   []*Item `json:"items"`
	History []*Item `json:"history"`
}

// Queue é a fila de pedidos. Fica em memória e é salva em disco a cada mudança.
type Queue struct {
	mu   sync.Mutex
	path string
	st   persisted
}

func newQueue(dir string) *Queue {
	q := &Queue{path: filepath.Join(dir, "queue.json"), st: persisted{Open: true, NextID: 1}}
	if b, err := os.ReadFile(q.path); err == nil {
		var p persisted
		if json.Unmarshal(b, &p) == nil {
			q.st = p
			if q.st.NextID < 1 {
				q.st.NextID = 1
			}
			for _, it := range append(append([]*Item{q.st.Current}, q.st.Items...), q.st.History...) {
				if it != nil && it.Song != nil {
					it.Song.prepare()
				}
			}
		}
	}
	return q
}

func (q *Queue) save() {
	b, err := json.Marshal(q.st)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(q.path), 0o755)
	_ = writeFileAtomic(q.path, b, 0o644)
}

type QueueView struct {
	Open    bool    `json:"open"`
	Current *Item   `json:"current"`
	Items   []*Item `json:"items"`
	History []*Item `json:"history"`
}

func (q *Queue) View() QueueView {
	q.mu.Lock()
	defer q.mu.Unlock()
	v := QueueView{Open: q.st.Open, Current: q.st.Current, Items: append([]*Item{}, q.st.Items...)}
	h := q.st.History
	if len(h) > 20 {
		h = h[len(h)-20:]
	}
	v.History = append([]*Item{}, h...)
	return v
}

func (q *Queue) IsOpen() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.st.Open
}

func (q *Queue) SetOpen(o bool) {
	q.mu.Lock()
	q.st.Open = o
	q.save()
	q.mu.Unlock()
}

func sameSong(a, b *Song) bool {
	return a != nil && b != nil && a.nArtist == b.nArtist && a.nTitle == b.nTitle
}

func (q *Queue) countUser(user string) int {
	n := 0
	for _, it := range q.st.Items {
		if strings.EqualFold(it.User, user) {
			n++
		}
	}
	return n
}

// Add coloca um pedido na fila e devolve a posição (1 = próxima).
// O streamer (broadcaster) e pedidos manuais ignoram abertura e limites.
func (q *Queue) Add(s *Song, user, platform string, roles Roles, r Rules) (*Item, int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	manual := platform == "manual"
	bypass := manual || roles.Broadcaster
	if !bypass {
		if !q.st.Open {
			return nil, 0, ErrClosed
		}
		if len(q.st.Items) >= r.MaxQueue {
			return nil, 0, ErrFull
		}
		if q.countUser(user) >= r.MaxPerUser {
			return nil, 0, ErrUserLimit
		}
	}
	if r.BlockDuplicates && !manual {
		if q.st.Current != nil && sameSong(q.st.Current.Song, s) {
			return nil, 0, ErrDuplicate
		}
		for _, it := range q.st.Items {
			if sameSong(it.Song, s) {
				return nil, 0, ErrDuplicate
			}
		}
	}
	it := &Item{ID: q.st.NextID, Song: s, User: user, Platform: platform, At: time.Now()}
	q.st.NextID++
	pos := len(q.st.Items)
	if r.PriorityRoles && roles.Priority() && !manual {
		it.Priority = true
		// entra depois do último pedido prioritário
		pos = 0
		for i, x := range q.st.Items {
			if x.Priority {
				pos = i + 1
			}
		}
	}
	q.st.Items = append(q.st.Items, nil)
	copy(q.st.Items[pos+1:], q.st.Items[pos:])
	q.st.Items[pos] = it
	q.save()
	return it, pos + 1, nil
}

func (q *Queue) finishCurrent() {
	if q.st.Current != nil {
		q.st.History = append(q.st.History, q.st.Current)
		if len(q.st.History) > 200 {
			q.st.History = q.st.History[len(q.st.History)-200:]
		}
		q.st.Current = nil
	}
}

// Next conclui a música atual e chama a primeira da fila.
func (q *Queue) Next() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.finishCurrent()
	if len(q.st.Items) > 0 {
		q.st.Current = q.st.Items[0]
		q.st.Items = q.st.Items[1:]
	}
	q.save()
}

// Done conclui a música atual sem chamar a próxima.
func (q *Queue) Done() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.finishCurrent()
	q.save()
}

// Play escolhe um pedido específico da fila para tocar agora.
func (q *Queue) Play(id int64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, it := range q.st.Items {
		if it.ID == id {
			q.finishCurrent()
			q.st.Current = it
			q.st.Items = append(q.st.Items[:i:i], q.st.Items[i+1:]...)
			q.save()
			return true
		}
	}
	return false
}

func (q *Queue) Remove(id int64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, it := range q.st.Items {
		if it.ID == id {
			q.st.Items = append(q.st.Items[:i:i], q.st.Items[i+1:]...)
			q.save()
			return true
		}
	}
	return false
}

// RemoveLastBy remove o pedido mais recente de um usuário.
func (q *Queue) RemoveLastBy(user string) *Item {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := len(q.st.Items) - 1; i >= 0; i-- {
		if strings.EqualFold(q.st.Items[i].User, user) {
			it := q.st.Items[i]
			q.st.Items = append(q.st.Items[:i:i], q.st.Items[i+1:]...)
			q.save()
			return it
		}
	}
	return nil
}

func (q *Queue) Move(id int64, dir int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, it := range q.st.Items {
		if it.ID == id {
			j := i + dir
			if j < 0 || j >= len(q.st.Items) {
				return false
			}
			q.st.Items[i], q.st.Items[j] = q.st.Items[j], q.st.Items[i]
			q.save()
			return true
		}
	}
	return false
}

func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.st.Items = nil
	q.save()
}
