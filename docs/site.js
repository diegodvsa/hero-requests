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
  async function loadSongs() {
    if (!info) { $('list').innerHTML = '<li class="msg">Abra este site pelo endereço do GitHub Pages, ou use ?repo=usuario/repositorio</li>'; return; }
    try {
      var r = await fetch('https://raw.githubusercontent.com/' + info.repo + '/' + info.branch + '/songs.json');
      if (!r.ok) throw new Error('HTTP ' + r.status);
      var d = await r.json();
      songs = (d.songs || []).map(function (s) {
        return { artist: s[0], title: s[1], album: s[2], genre: s[3], charter: s[4], year: s[5], n: norm(s[0] + ' ' + s[1]) };
      });
      var genres = {};
      songs.forEach(function (s) { if (s.genre) genres[s.genre] = 1; });
      var sel = $('genre');
      Object.keys(genres).sort().forEach(function (g) {
        var o = document.createElement('option'); o.value = g; o.textContent = g; sel.appendChild(o);
      });
      renderSongs();
    } catch (e) {
      $('list').innerHTML = '<li class="msg">A lista de músicas ainda não foi publicada (' + esc(e.message) + ').</li>';
    }
  }

  function renderSongs() {
    var q = norm($('q').value), g = $('genre').value;
    var toks = q ? q.split(' ') : [];
    var res = songs.filter(function (s) {
      if (g && s.genre !== g) return false;
      return toks.every(function (t) { return s.n.indexOf(t) >= 0; });
    });
    $('count').textContent = res.length.toLocaleString('pt-BR') + ' de ' + songs.length.toLocaleString('pt-BR') + ' músicas';
    $('list').innerHTML = res.slice(0, shown).map(function (s, i) {
      var meta = [s.album, s.genre, s.year].filter(Boolean).join(' · ');
      return '<li><div><b>' + esc(s.title) + '</b> · ' + esc(s.artist) + '<span class="meta">' + esc(meta) + '</span></div>' +
        '<button class="btn sm" type="button" data-i="' + i + '">Copiar pedido</button></li>';
    }).join('') || '<li class="msg">Nenhuma música encontrada.</li>';
    $('more').hidden = res.length <= shown;
    $('list')._res = res;
  }

  $('q').addEventListener('input', function () { shown = PAGE; renderSongs(); });
  $('genre').addEventListener('change', function () { shown = PAGE; renderSongs(); });
  $('more').addEventListener('click', function () { shown += PAGE; renderSongs(); });
  $('list').addEventListener('click', function (e) {
    var b = e.target.closest('button[data-i]'); if (!b) return;
    var s = $('list')._res[+b.dataset.i];
    var cmd = '!pedir ' + (s.artist ? s.artist + ' - ' : '') + s.title;
    var done = function (ok) { b.textContent = ok ? 'Copiado!' : 'Copie: ' + cmd; setTimeout(function () { b.textContent = 'Copiar pedido'; }, 2500); };
    if (navigator.clipboard) navigator.clipboard.writeText(cmd).then(function () { done(true); }, function () { done(false); });
    else done(false);
  });

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
