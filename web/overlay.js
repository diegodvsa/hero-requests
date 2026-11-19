// Overlay do OBS. Opções por endereço: /overlay?items=5&accent=%23ff8a3d&pos=left&hint=0
(function () {
  var qs = new URLSearchParams(location.search);
  if (qs.get('pos') === 'left') document.body.classList.add('left');
  var wrap = document.getElementById('wrap');
  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function label(s) { return s.artist ? s.artist + ' - ' + s.title : s.title; }
  function badge(p) {
    var l = p === 'twitch' ? 'T' : p === 'kick' ? 'K' : '★';
    return '<span class="src ' + esc(p) + '">' + l + '</span>';
  }
  function render(st) {
    var ov = st.overlay || {};
    var accent = qs.get('accent') || ov.accent;
    if (accent) document.documentElement.style.setProperty('--accent', accent);
    var n = parseInt(qs.get('items') || ov.items || 5, 10);
    var q = st.queue || {};
    var html = '';
    if (q.current) {
      html += '<div class="now"><div class="lbl">Tocando agora</div><div class="t">' + esc(q.current.song.title) +
        '</div><div class="sub">' + esc(q.current.song.artist) + ' · pedido por ' + esc(q.current.user) + '</div></div>';
    }
    var items = q.items || [];
    if (items.length) {
      html += '<div class="q"><div class="lbl">Próximas · ' + items.length + ' na fila</div>';
      items.slice(0, n).forEach(function (it, i) {
        html += '<div class="row"><span class="n">' + (i + 1) + '</span><span class="nm">' + esc(label(it.song)) +
          '</span><span class="by">' + esc(it.user) + '</span>' + badge(it.platform) + '</div>';
      });
      if (items.length > n) html += '<div class="sub">+' + (items.length - n) + ' na fila</div>';
      html += '</div>';
    }
    if (qs.get('hint') !== '0') {
      html += q.open === false
        ? '<div class="hint">Pedidos fechados no momento</div>'
        : '<div class="hint">Peça com <code>!pedir artista - música</code></div>';
    }
    wrap.innerHTML = html;
  }
  function connect() {
    var es = new EventSource('/api/events');
    es.onmessage = function (e) { try { render(JSON.parse(e.data)); } catch (_) {} };
    es.onerror = function () { es.close(); setTimeout(connect, 2000); };
  }
  connect();
})();
