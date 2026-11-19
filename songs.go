package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// bom é a marca de ordem de bytes que alguns editores colocam no início de arquivos UTF-8.
const bom = "\xef\xbb\xbf"

// Song é uma música da biblioteca do jogo.
type Song struct {
	ID      int    `json:"id"`
	Artist  string `json:"artist"`
	Title   string `json:"title"`
	Album   string `json:"album,omitempty"`
	Genre   string `json:"genre,omitempty"`
	Charter string `json:"charter,omitempty"`
	Year    string `json:"year,omitempty"`

	nArtist string
	nTitle  string
}

func (s *Song) Label() string {
	if s.Artist == "" {
		return s.Title
	}
	return s.Artist + " - " + s.Title
}

func (s *Song) prepare() {
	s.nArtist = normalize(s.Artist)
	s.nTitle = normalize(s.Title)
}

var accentFold = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n", "ß", "ss",
)

// normalize deixa minúsculo, sem acento e sem pontuação.
func normalize(s string) string {
	s = accentFold.Replace(strings.ToLower(s))
	var b strings.Builder
	space := true
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// Library guarda a lista de músicas em memória.
type Library struct {
	mu    sync.RWMutex
	songs []*Song
}

func (l *Library) Set(songs []*Song) {
	l.mu.Lock()
	l.songs = songs
	l.mu.Unlock()
}

func (l *Library) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.songs)
}

func (l *Library) All() []*Song {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]*Song(nil), l.songs...)
}

func (l *Library) ByID(id int) *Song {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if id < 0 || id >= len(l.songs) {
		return nil
	}
	return l.songs[id]
}

type Scored struct {
	Song  *Song
	Score int
}

// Search encontra músicas. Aceita "artista - música", "música - artista"
// ou apenas palavras soltas.
func (l *Library) Search(q string, limit int) []Scored {
	l.mu.RLock()
	defer l.mu.RUnlock()
	nq := normalize(q)
	if nq == "" {
		return nil
	}
	var artistQ, titleQ string
	if i := strings.Index(q, " - "); i > 0 {
		artistQ, titleQ = normalize(q[:i]), normalize(q[i+3:])
	}
	toks := strings.Fields(nq)
	var out []Scored
	for _, s := range l.songs {
		sc := 0
		if artistQ != "" && titleQ != "" {
			switch {
			case strings.Contains(s.nArtist, artistQ) && strings.Contains(s.nTitle, titleQ):
				sc = 100
				if s.nTitle == titleQ {
					sc += 100
				}
				if s.nArtist == artistQ {
					sc += 50
				}
			case strings.Contains(s.nArtist, titleQ) && strings.Contains(s.nTitle, artistQ):
				// "música - artista"
				sc = 90
				if s.nTitle == artistQ {
					sc += 100
				}
			}
		}
		if sc == 0 {
			sc = looseScore(s, nq, toks)
		}
		if sc > 0 {
			out = append(out, Scored{s, sc})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return len(out[i].Song.nTitle) < len(out[j].Song.nTitle)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func looseScore(s *Song, nq string, toks []string) int {
	hay := " " + s.nArtist + " " + s.nTitle + " "
	score := 0
	for _, t := range toks {
		switch {
		case strings.Contains(" "+s.nTitle+" ", " "+t+" "):
			score += 4
		case strings.Contains(" "+s.nArtist+" ", " "+t+" "):
			score += 3
		case strings.Contains(hay, " "+t):
			score += 2
		case strings.Contains(hay, t):
			score += 1
		default:
			return 0
		}
	}
	switch {
	case s.nTitle == nq:
		score += 100
	case s.nArtist+" "+s.nTitle == nq, s.nTitle+" "+s.nArtist == nq:
		score += 120
	case strings.HasPrefix(s.nTitle, nq):
		score += 20
	}
	return score
}

// ---------- carregamento ----------

func loadSongs(cfg SongsCfg) ([]*Song, error) {
	if cfg.Path == "" {
		return nil, errors.New("caminho das músicas não configurado")
	}
	var raw []*Song
	var err error
	if cfg.Mode == "folder" {
		raw, err = scanFolder(cfg.Path)
	} else {
		raw, err = loadJSON(cfg.Path)
	}
	if err != nil {
		return nil, err
	}
	// remove duplicatas (mesma música com charters diferentes)
	seen := map[string]bool{}
	var out []*Song
	for _, s := range raw {
		s.prepare()
		if s.nTitle == "" {
			continue
		}
		k := s.nArtist + "|" + s.nTitle
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].nArtist != out[j].nArtist {
			return out[i].nArtist < out[j].nArtist
		}
		return out[i].nTitle < out[j].nTitle
	})
	for i, s := range out {
		s.ID = i
	}
	if len(out) == 0 {
		return nil, errors.New("nenhuma música encontrada nesse caminho")
	}
	return out, nil
}

var keyAliases = map[string][]string{
	"artist":  {"artist", "artists", "artistname", "band", "performer"},
	"title":   {"name", "title", "songname", "song", "track", "trackname"},
	"album":   {"album", "albumname"},
	"genre":   {"genre", "genres"},
	"charter": {"charter", "charters", "frets", "author", "chartedby"},
	"year":    {"year", "releaseyear"},
}

// loadJSON lê a exportação de músicas do jogo. O formato é tolerante:
// aceita uma lista de objetos, ou um objeto com a lista dentro de
// "songs", "entries", "tracks" etc., e vários nomes de campo.
func loadJSON(path string) ([]*Song, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("não consegui abrir o arquivo: %w", err)
	}
	b = []byte(strings.TrimPrefix(string(b), bom))
	var root any
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("o arquivo não é um JSON válido: %w", err)
	}
	items := findItems(root)
	var songs []*Song
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		flat := flatten(m)
		s := &Song{
			Artist:  pick(flat, "artist"),
			Title:   pick(flat, "title"),
			Album:   pick(flat, "album"),
			Genre:   pick(flat, "genre"),
			Charter: pick(flat, "charter"),
			Year:    pick(flat, "year"),
		}
		if s.Title != "" {
			songs = append(songs, s)
		}
	}
	if len(songs) == 0 {
		return nil, errors.New("o JSON não tem músicas reconhecíveis (esperado: campos como artist e name/title)")
	}
	return songs, nil
}

var listKeys = []string{"songs", "entries", "tracks", "items", "data", "songlist", "library"}

func findItems(root any) []any {
	switch v := root.(type) {
	case []any:
		return v
	case map[string]any:
		for k, val := range v {
			lk := strings.ToLower(k)
			for _, want := range listKeys {
				if lk == want {
					if arr := findItems(val); len(arr) > 0 {
						return arr
					}
				}
			}
		}
		// objeto de objetos: {"id1": {...}, "id2": {...}}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var arr []any
		for _, k := range keys {
			if _, ok := v[k].(map[string]any); ok {
				arr = append(arr, v[k])
			}
		}
		return arr
	}
	return nil
}

// flatten junta campos do objeto e de um nível de sub-objetos (ex.: "metadata").
func flatten(m map[string]any) map[string]string {
	out := map[string]string{}
	var walk func(m map[string]any, depth int)
	walk = func(m map[string]any, depth int) {
		for k, v := range m {
			lk := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(k))
			switch t := v.(type) {
			case string:
				if _, dup := out[lk]; !dup {
					out[lk] = strings.TrimSpace(t)
				}
			case float64:
				if _, dup := out[lk]; !dup {
					out[lk] = strconv.FormatFloat(t, 'f', -1, 64)
				}
			case []any:
				var parts []string
				for _, e := range t {
					if s, ok := e.(string); ok {
						parts = append(parts, s)
					}
				}
				if len(parts) > 0 {
					if _, dup := out[lk]; !dup {
						out[lk] = strings.Join(parts, ", ")
					}
				}
			case map[string]any:
				if depth < 1 {
					walk(t, depth+1)
				}
			}
		}
	}
	walk(m, 0)
	return out
}

func pick(flat map[string]string, field string) string {
	for _, a := range keyAliases[field] {
		if v := flat[a]; v != "" {
			return v
		}
	}
	return ""
}

// scanFolder lê arquivos song.ini (formato do Clone Hero/YARG).
func scanFolder(root string) ([]*Song, error) {
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return nil, errors.New("pasta não encontrada")
	}
	var songs []*Song
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(d.Name(), "song.ini") {
			return nil
		}
		if s := parseIni(p); s != nil {
			songs = append(songs, s)
		}
		return nil
	})
	return songs, nil
}

func parseIni(path string) *Song {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	s := &Song{}
	sc := bufio.NewScanner(f)
	inSong := false
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), bom))
		if strings.HasPrefix(line, "[") {
			inSong = strings.EqualFold(line, "[song]")
			continue
		}
		if !inSong {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		switch k {
		case "name":
			s.Title = v
		case "artist":
			s.Artist = v
		case "album":
			s.Album = v
		case "genre":
			s.Genre = v
		case "charter", "frets":
			if s.Charter == "" {
				s.Charter = v
			}
		case "year":
			s.Year = strings.TrimPrefix(v, ", ")
		}
	}
	if s.Title == "" {
		return nil
	}
	return s
}
