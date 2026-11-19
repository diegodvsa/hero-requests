// Site estático: lê songs.json e queue.json da branch "data" do próprio repositório.
(function () {
  'use strict';
  var PAGE = 100;
  var $ = function (id) { return document.getElementById(id); };
  var songs = [], shown = PAGE, timer = null;

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function norm(s) {
    return String(s || '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, ' ').trim();
  }
  function repoInfo() {
    var q = new URLSearchParams(location.search);
    if (q.get('repo')) return { repo: q.get('repo'), branch: q.get('branch') || 'data' };
    var m = location.hostname.match(/^(.+)\.github\.io$/i);
    var seg = location.pathname.split('/')[1];
    if (m && seg) return { repo: m[1] + '/' + seg, branch: 'data' };
    return null;
  }
  var info = repoInfo();

  function setStatus(cls, text) {
    $('status').className = 'status ' + cls;
    $('status-text').textContent = text;
  }

  // ---------- músicas ----------
  var F = { artist: '', album: '', source: '', genre: '' };

  function fillSelect(id, wrapId, values) {
    var list = Object.keys(values).sort(function (a, b) { return a.localeCompare(b, 'pt-BR'); });
    var sel = $(id);
    list.forEach(function (v) { var o = document.createElement('option'); o.value = v; o.textContent = v; sel.appendChild(o); });
    $(wrapId).hidden = list.length === 0; // esconde o filtro se a lista não tem esse dado
  }
  function fillDatalist(id, values) {
    var dl = $(id);
    Object.keys(values).sort(function (a, b) { return a.localeCompare(b, 'pt-BR'); }).forEach(function (v) {
      var o = document.createElement('option'); o.value = v; dl.appendChild(o);
    });
  }

  async function loadSongs() {
    if (!info) { $('list').innerHTML = '<li class="msg">Abra este site pelo endereço do GitHub Pages, ou use ?repo=usuario/repositorio</li>'; return; }
    try {
      var r = await fetch('https://raw.githubusercontent.com/' + info.repo + '/' + info.branch + '/songs.json');
      if (!r.ok) throw new Error('HTTP ' + r.status);
      var d = await r.json();
      songs = (d.songs || []).map(function (s) {
        return {
          artist: s[0] || '', title: s[1] || '', album: s[2] || '', genre: s[3] || '', charter: s[4] || '', year: s[5] || '', source: s[6] || '',
          n: norm(s[0] + ' ' + s[1]), nArtist: norm(s[0]), nAlbum: norm(s[2])
        };
      });
      var genres = {}, sources = {}, artists = {}, albums = {};
      songs.forEach(function (s) {
        if (s.genre) genres[s.genre] = 1;
        if (s.source) sources[s.source] = 1;
        if (s.artist) artists[s.artist] = 1;
        if (s.album) albums[s.album] = 1;
      });
      fillSelect('f-genre', 'w-genre', genres);
      fillSelect('f-source', 'w-source', sources);
      fillDatalist('dl-artist', artists);
      fillDatalist('dl-album', albums);
      $('f-album').parentNode.hidden = Object.keys(albums).length === 0;
      readHash();
      renderSongs();
    } catch (e) {
      $('list').innerHTML = '<li class="msg">A lista de músicas ainda não foi publicada (' + esc(e.message) + ').</li>';
    }
  }

  function $f(id) { return document.getElementById(id); }
  function syncInputs() {
    $f('f-artist').value = F.artist; $f('f-album').value = F.album;
    $f('f-source').value = F.source; $f('f-genre').value = F.genre;
  }
  function active() { return F.artist || F.album || F.source || F.genre || $('q').value.trim(); }

  function renderSongs() {
    var q = norm($('q').value), toks = q ? q.split(' ') : [];
    var fa = norm(F.artist), fb = norm(F.album);
    var res = songs.filter(function (s) {
      if (F.source && s.source !== F.source) return false;
      if (F.genre && s.genre !== F.genre) return false;
      if (fa && s.nArtist.indexOf(fa) < 0) return false;
      if (fb && s.nAlbum.indexOf(fb) < 0) return false;
      return toks.every(function (t) { return s.n.indexOf(t) >= 0; });
    });
    $('count').textContent = res.length.toLocaleString('pt-BR') + ' de ' + songs.length.toLocaleString('pt-BR') + ' músicas';
    $('clear').hidden = !active();
    $('list').innerHTML = res.slice(0, shown).map(function (s, i) {
      var parts = [];
      if (s.album) parts.push('<a data-f="album" data-v="' + esc(s.album) + '" title="Filtrar por este álbum">' + esc(s.album) + '</a>');
      if (s.source) parts.push('<a data-f="source" data-v="' + esc(s.source) + '" title="Filtrar por esta fonte">' + esc(s.source) + '</a>');
      if (s.genre) parts.push('<a data-f="genre" data-v="' + esc(s.genre) + '" title="Filtrar por este gênero">' + esc(s.genre) + '</a>');
      if (s.year) parts.push(esc(s.year));
      return '<li><div><b>' + esc(s.title) + '</b> · <span class="meta" style="display:inline"><a data-f="artist" data-v="' + esc(s.artist) + '" title="Filtrar por este artista">' + esc(s.artist) + '</a></span>' +
        '<span class="meta">' + parts.join(' · ') + '</span></div>' +
        '<button class="btn sm" type="button" data-i="' + i + '">Copiar pedido</button></li>';
    }).join('') || '<li class="msg">Nenhuma música encontrada.</li>';
    $('more').hidden = res.length <= shown;
    $('list')._res = res;
  }

  function change() { shown = PAGE; renderSongs(); writeHash(); }
  $('q').addEventListener('input', change);
  [['f-artist', 'artist'], ['f-album', 'album']].forEach(function (p) {
    $f(p[0]).addEventListener('input', function (e) { F[p[1]] = e.target.value; change(); });
  });
  [['f-source', 'source'], ['f-genre', 'genre']].forEach(function (p) {
    $f(p[0]).addEventListener('change', function (e) { F[p[1]] = e.target.value; change(); });
  });
  $('clear').addEventListener('click', function () {
    F = { artist: '', album: '', source: '', genre: '' }; $('q').value = ''; syncInputs(); change();
  });
  $('more').addEventListener('click', function () { shown += PAGE; renderSongs(); });
  $('list').addEventListener('click', function (e) {
    var a = e.target.closest('a[data-f]');
    if (a) { F[a.dataset.f] = a.dataset.v; syncInputs(); change(); window.scrollTo(0, 0); return; }
    var b = e.target.closest('button[data-i]'); if (!b) return;
    var s = $('list')._res[+b.dataset.i];
    var cmd = '!pedir ' + (s.artist ? s.artist + ' - ' : '') + s.title;
    var done = function (ok) { b.textContent = ok ? 'Copiado!' : 'Copie: ' + cmd; setTimeout(function () { b.textContent = 'Copiar pedido'; }, 2500); };
    if (navigator.clipboard) navigator.clipboard.writeText(cmd).then(function () { done(true); }, function () { done(false); });
    else done(false);
  });

  // os filtros ficam no endereço (#artist=...&source=...) para compartilhar a busca
  function writeHash() {
    var p = new URLSearchParams();
    Object.keys(F).forEach(function (k) { if (F[k]) p.set(k, F[k]); });
    if ($('q').value.trim()) p.set('q', $('q').value.trim());
    var s = p.toString();
    try { history.replaceState(null, '', s ? '#' + s : location.pathname + location.search); } catch (_) {}
  }
  window.addEventListener('hashchange', function () { if (songs.length) { readHash(); shown = PAGE; renderSongs(); } });
  function readHash() {
    var p = new URLSearchParams(location.hash.slice(1));
    Object.keys(F).forEach(function (k) { F[k] = p.get(k) || ''; });
    $('q').value = p.get('q') || '';
    syncInputs();
  }

  // ---------- fila ----------
  async function loadQueue() {
    if (!info) return;
    try {
      var r = await fetch('https://api.github.com/repos/' + info.repo + '/contents/queue.json?ref=' + encodeURIComponent(info.branch), {
        headers: { Accept: 'application/vnd.github.raw+json' }
      });
      if (r.status === 403 || r.status === 429) throw new Error('limite de consultas do GitHub, tente em alguns minutos');
      if (!r.ok) throw new Error('HTTP ' + r.status);
      var d = await r.json();
      if (d.channel) $('title').textContent = 'Pedidos de ' + d.channel;
      setStatus(d.open ? 'open' : 'closed', d.open ? 'pedidos abertos' : 'pedidos fechados');
      $('now').className = 'card' + (d.current ? ' live' : '');
      $('now').innerHTML = d.current
        ? '<b>' + esc(d.current.title) + '</b><br><span class="muted">' + esc(d.current.artist) + ' · pedido de ' + esc(d.current.user) + '</span>'
        : '<span class="muted">Nada tocando agora.</span>';
      $('queue').innerHTML = d.queue.length ? d.queue.map(function (it, i) {
        return '<li><span class="n">' + (i + 1) + '</span><div><b>' + esc(it.title) + '</b><small>' + esc(it.artist) + ' · ' + esc(it.user) + '</small></div></li>';
      }).join('') : '<li class="muted">Fila vazia.</li>';
      $('updated').textContent = 'Fila atualizada em ' + new Date(d.updated).toLocaleTimeString('pt-BR');
    } catch (e) {
      setStatus('', 'fila indisponível');
      $('updated').textContent = e.message;
    }
  }

  function poll() {
    clearTimeout(timer);
    if (!document.hidden) loadQueue();
    timer = setTimeout(poll, 60000); // 60 consultas/h por visitante, dentro do limite do GitHub
  }
  $('refresh').addEventListener('click', loadQueue);
  document.addEventListener('visibilitychange', function () { if (!document.hidden) poll(); });

  loadSongs();
  poll();
})();
