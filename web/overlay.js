// Overlay do OBS. A aparência vem das configurações (state.overlay) e atualiza ao vivo.
// Em /overlay?preview=1 usa dados de exemplo e recebe a aparência por postMessage (editor do painel).
(function () {
  'use strict';
  var preview = new URLSearchParams(location.search).get('preview') === '1';
  var wrap = document.getElementById('wrap');
  var root = document.documentElement;
  var cfg = null, queue = null, gfLoaded = '';

  var SAMPLE = {
    open: true,
    current: { song: { artist: 'Metallica', title: 'Master of Puppets' }, user: 'rafa_drums', platform: 'twitch' },
    items: [
      { song: { artist: 'Foo Fighters', title: 'Everlong' }, user: 'lu_guitar', platform: 'kick' },
      { song: { artist: 'Slipknot', title: 'Duality' }, user: 'dudu_br', platform: 'twitch' },
      { song: { artist: 'Paramore', title: 'Misery Business' }, user: 'carol', platform: 'kick' },
      { song: { artist: 'Legião Urbana', title: 'Tempo Perdido' }, user: 'mari_k', platform: 'twitch' },
      { song: { artist: 'Queen', title: 'Bohemian Rhapsody' }, user: 'joao99', platform: 'twitch' },
      { song: { artist: 'Nirvana', title: 'Smells Like Teen Spirit' }, user: 'bia', platform: 'kick' }
    ]
  };

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function label(s) { return s.artist ? s.artist + ' - ' + s.title : s.title; }
  function badge(p) {
    return '<span class="src ' + esc(p) + '">' + (p === 'twitch' ? 'T' : p === 'kick' ? 'K' : '★') + '</span>';
  }
  function rgba(hex, pct) {
    var n = parseInt(hex.slice(1), 16);
    return 'rgba(' + (n >> 16 & 255) + ',' + (n >> 8 & 255) + ',' + (n & 255) + ',' + (pct / 100) + ')';
  }
  function fontName(f) { return String(f).split(',')[0].replace(/["']/g, '').trim(); }

  function applyStyle(o) {
    var set = function (k, v) { root.style.setProperty(k, v); };
    set('--accent', o.accent); set('--text', o.textColor); set('--muted', o.mutedColor);
    set('--bg', rgba(o.bgColor, o.bgOpacity)); set('--border', rgba(o.borderColor, o.borderOpacity));
    set('--fs', o.fontSize + 'px'); set('--ts', o.titleSize + 'px'); set('--ls', Math.max(9, Math.round(o.fontSize * 0.75)) + 'px');
    set('--radius', o.radius + 'px'); set('--bw', o.borderWidth + 'px'); set('--pad', o.padding + 'px');
    set('--gap', o.gap + 'px'); set('--blur', o.blur + 'px'); set('--margin', o.margin + 'px'); set('--width', o.width + 'px');
    var fam = o.fontFamily;
    if (o.fontGoogle) {
      var name = fontName(fam);
      if (name && name !== gfLoaded) {
        gfLoaded = name;
        var l = document.createElement('link');
        l.rel = 'stylesheet';
        l.href = 'https://fonts.googleapis.com/css2?family=' + encodeURIComponent(name).replace(/%20/g, '+') + ':wght@400;700&display=swap';
        document.head.appendChild(l);
      }
      if (fam.indexOf(',') < 0) fam = '"' + name + '", system-ui, sans-serif';
    }
    set('--font', fam);
    wrap.className = o.corner;
    document.getElementById('custom').textContent = o.customCss || '';
  }

  function hint(text) {
    return esc(text).replace(/(![\wÀ-ÿ]+)/g, '<b>$1</b>');
  }

  function render() {
    if (!cfg || !queue) return;
    var o = cfg, q = queue, h = '';
    if (o.showNow && q.current) {
      h += '<div class="box now' + (o.accentBar ? ' bar' : '') + '">' +
        (o.showLabels ? '<div class="lbl">' + esc(o.nowLabel) + '</div>' : '') +
        '<div class="t">' + esc(q.current.song.title) + '</div><div class="sub">' + esc(q.current.song.artist) +
        (o.showRequester ? ' · pedido por ' + esc(q.current.user) : '') + '</div></div>';
    }
    var items = q.items || [];
    if (o.showQueue && items.length) {
      h += '<div class="box">' +
        (o.showLabels ? '<div class="lbl">' + esc(o.queueLabel) + ' · ' + items.length + ' na fila</div>' : '');
      items.slice(0, o.items).forEach(function (it, i) {
        h += '<div class="row"><span class="n">' + (i + 1) + '</span><span class="nm">' + esc(label(it.song)) + '</span>' +
          (o.showRequester ? '<span class="by">' + esc(it.user) + '</span>' : '') +
          (o.showBadges ? badge(it.platform) : '') + '</div>';
      });
      if (items.length > o.items) h += '<div class="sub">+' + (items.length - o.items) + ' na fila</div>';
      h += '</div>';
    }
    if (o.showHint) {
      h += '<div class="box hint">' + (q.open === false ? 'Pedidos fechados no momento' : hint(o.hintText)) + '</div>';
    }
    wrap.innerHTML = h;
  }

  if (preview) {
    queue = SAMPLE;
    window.addEventListener('message', function (e) {
      if (e.origin !== location.origin || !e.data || e.data.type !== 'overlay') return;
      cfg = e.data.cfg; applyStyle(cfg); render();
    });
    window.parent.postMessage({ type: 'overlay-ready' }, location.origin);
  } else {
    var connect = function () {
      var es = new EventSource('/api/events');
      es.onmessage = function (e) {
        try {
          var st = JSON.parse(e.data);
          cfg = st.overlay; queue = st.queue;
          applyStyle(cfg); render();
        } catch (_) {}
      };
      es.onerror = function () { es.close(); setTimeout(connect, 2000); };
    };
    connect();
  }
})();
