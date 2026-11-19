// Painel do Hero Requests: fila, configurações e assistente inicial.
(function () {
  'use strict';
  var $ = function (s, el) { return (el || document).querySelector(s); };
  var state = null, cfg = null, mode = 'panel', step = 0;
  var STEPS = 5;

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function label(s) { return s.artist ? s.artist + ' - ' + s.title : s.title; }
  function badge(p) {
    return '<span class="src ' + esc(p) + '" title="' + esc(p) + '">' + (p === 'twitch' ? 'T' : p === 'kick' ? 'K' : '★') + '</span>';
  }
  function toast(msg) {
    var t = $('#toast'); t.textContent = msg; t.hidden = false;
    clearTimeout(toast._t); toast._t = setTimeout(function () { t.hidden = true; }, 3500);
  }
  async function api(path, method, body) {
    var r = await fetch(path, {
      method: method || 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body)
    });
    var data = {}; try { data = await r.json(); } catch (_) {}
    if (!r.ok) throw new Error(data.error || ('Erro ' + r.status));
    return data;
  }

  // ---------- eventos ----------
  function connect() {
    var es = new EventSource('/api/events');
    es.onmessage = function (e) {
      var first = state === null;
      state = JSON.parse(e.data);
      if (first) {
        if (!state.configured) openSettings(true); else showPanel();
      }
      if (mode === 'panel') renderPanel();
      renderChips();
    };
    es.onerror = function () { es.close(); setTimeout(connect, 2000); };
  }

  // ---------- painel ----------
  function showPanel() {
    mode = 'panel';
    $('#panel').hidden = false; $('#settings').hidden = true;
    $('#btn-settings').hidden = false;
    renderPanel(); renderChips();
  }

  function chip(name, s) {
    if (!s.enabled) return '<span class="chip"><i class="dot"></i>' + name + ' desligado</span>';
    var d = s.detail ? ' · ' + s.detail : '';
    return '<span class="chip" title="' + esc(s.detail) + '"><i class="dot ' + esc(s.state) + '"></i>' + name + ' ' + esc(s.state) + (s.state === 'erro' ? '' : esc(d)) + '</span>';
  }
  function renderChips() {
    if (!state) return;
    var h = chip('Twitch', state.twitch) + chip('Kick', state.kick);
    h += '<span class="chip">' + (state.songs ? state.songs.toLocaleString('pt-BR') + ' músicas' : 'sem músicas') + '</span>';
    $('#chips').innerHTML = h;
  }

  function itemRow(it, i) {
    return '<div class="item">' +
      '<span class="pos">' + (i + 1) + '</span>' + badge(it.platform) +
      '<div class="nm"><b>' + esc(it.song.title) + (it.priority ? ' <span class="badge">PRIORIDADE</span>' : '') + '</b><span>' + esc(it.song.artist) + ' · ' + esc(it.user) + '</span></div>' +
      '<button class="btn p sm" data-act="play" data-id="' + it.id + '" type="button">Tocar</button>' +
      '<button class="btn sm" data-act="up" data-id="' + it.id + '" type="button" aria-label="Subir">↑</button>' +
      '<button class="btn sm" data-act="down" data-id="' + it.id + '" type="button" aria-label="Descer">↓</button>' +
      '<button class="btn sm" data-act="remove" data-id="' + it.id + '" type="button" aria-label="Remover">✕</button></div>';
  }

  function renderPanel() {
    if (!state) return;
    var q = state.queue, h = '';
    if (state.songsErr) {
      h += '<div class="alert">' + esc(state.songsErr) + ' <button class="btn sm" data-act="settings" type="button">Abrir configurações</button></div>';
    }
    if (q.current) {
      h += '<div class="card live now"><div style="flex:1;min-width:150px"><div class="t">' + esc(q.current.song.title) +
        '</div><div class="mute">' + esc(q.current.song.artist) + ' · ' + esc(q.current.user) + '</div></div>' +
        '<button class="btn p" data-act="next" type="button">Concluir e chamar próxima</button>' +
        '<button class="btn" data-act="done" type="button">Concluir</button></div>';
    } else {
      h += '<div class="card now"><div style="flex:1" class="mute">Nada tocando.</div>' +
        '<button class="btn p" data-act="next" type="button"' + (q.items.length ? '' : ' disabled') + '>Chamar próxima</button></div>';
    }
    $('#now').innerHTML = h;
    $('#q-title').textContent = 'Fila · ' + q.items.length + ' pedido' + (q.items.length === 1 ? '' : 's');
    $('#queue').innerHTML = q.items.length
      ? q.items.map(itemRow).join('')
      : '<div class="empty">Ninguém pediu ainda. Os pedidos do chat aparecem aqui.</div>';

    var ob = $('#btn-open');
    ob.textContent = q.open ? 'Aberto · fechar' : 'Fechado · abrir';
    ob.className = 'btn sm' + (q.open ? '' : ' p');
    $('#r-max').textContent = state.rules.maxPerUser;
    $('#r-prio').textContent = state.rules.priorityRoles ? 'sim' : 'não';
    $('#ov-url').textContent = location.origin + '/overlay';

    $('#log').innerHTML = state.log.length ? state.log.slice().reverse().map(function (l) {
      return '<div><span>' + esc(l.time) + '</span> ' + badge(l.platform) + ' <b>' + esc(l.user) + '</b> ' + esc(l.text) +
        ' <span class="' + (l.ok ? 'ok' : 'no') + '">' + (l.ok ? '✓ ' : '✗ ') + esc(l.result) + '</span></div>';
    }).join('') : '<span>Nenhum comando ainda.</span>';

    var sb = '';
    if (state.site.enabled) {
      sb = '<div class="h">Site no GitHub</div><div class="card"><div class="set"><span>' +
        (state.site.error ? '<span class="test-err">Erro: ' + esc(state.site.error) + '</span>' : (state.site.lastOk ? 'Publicado às ' + esc(state.site.lastOk) : 'Aguardando publicação')) +
        '</span><button class="btn sm" data-act="publish" type="button">Publicar agora</button></div>' +
        (state.site.url ? '<div class="url">' + esc(state.site.url) + '</div>' : '') + '</div>';
    }
    $('#site-box').innerHTML = sb;
  }

  async function doAct(act, id) {
    try {
      if (act === 'play') await api('/api/queue/play', 'POST', { id: +id });
      else if (act === 'remove') await api('/api/queue/remove', 'POST', { id: +id });
      else if (act === 'up') await api('/api/queue/move', 'POST', { id: +id, dir: -1 });
      else if (act === 'down') await api('/api/queue/move', 'POST', { id: +id, dir: 1 });
      else if (act === 'next') await api('/api/queue/next');
      else if (act === 'done') await api('/api/queue/done');
      else if (act === 'settings') openSettings(false);
      else if (act === 'publish') { toast('Publicando…'); await api('/api/publish'); toast('Publicado no GitHub'); }
    } catch (e) { toast(e.message); }
  }

  document.addEventListener('click', function (e) {
    var b = e.target.closest('[data-act]');
    if (b) doAct(b.dataset.act, b.dataset.id);
  });
  $('#btn-settings').addEventListener('click', function () { openSettings(false); });
  $('#btn-clear').addEventListener('click', function () {
    if (state && state.queue.items.length && confirm('Remover todos os pedidos da fila?')) api('/api/queue/clear').catch(function (e) { toast(e.message); });
  });
  $('#btn-open').addEventListener('click', function () { api('/api/open', 'POST', { open: !state.queue.open }); });
  $('#btn-copy-ov').addEventListener('click', function () {
    var t = location.origin + '/overlay';
    (navigator.clipboard ? navigator.clipboard.writeText(t) : Promise.reject()).then(function () { toast('Link copiado'); }, function () { toast(t); });
  });
  $('#sim-form').addEventListener('submit', function (e) {
    e.preventDefault();
    var i = $('#sim-text'); var t = i.value.trim(); if (!t) return;
    api('/api/simulate', 'POST', { text: t, user: 'viewer_teste' }).then(function () { i.value = ''; }, function (er) { toast(er.message); });
  });
  var addTimer;
  $('#add-q').addEventListener('input', function (e) {
    clearTimeout(addTimer);
    var v = e.target.value.trim();
    if (!v) { $('#add-results').innerHTML = ''; return; }
    addTimer = setTimeout(async function () {
      try {
        var r = await api('/api/songs/search?q=' + encodeURIComponent(v), 'GET');
        $('#add-results').innerHTML = r.length ? r.map(function (s) {
          return '<button type="button" data-song="' + s.id + '">' + esc(s.label) + '</button>';
        }).join('') : '<div class="mute">Nada encontrado.</div>';
      } catch (er) { toast(er.message); }
    }, 200);
  });
  $('#add-results').addEventListener('click', async function (e) {
    var b = e.target.closest('[data-song]'); if (!b) return;
    await api('/api/queue/add', 'POST', { songId: +b.dataset.song, user: 'streamer' }).catch(function (er) { toast(er.message); });
    $('#add-q').value = ''; $('#add-results').innerHTML = '';
  });

  // ---------- configurações / assistente ----------
  async function openSettings(wizard) {
    try { cfg = await api('/api/config', 'GET'); } catch (e) { toast(e.message); return; }
    mode = wizard ? 'wizard' : 'settings'; step = 0;
    $('#panel').hidden = true; $('#settings').hidden = false;
    $('#btn-settings').hidden = wizard;
    buildSettings();
  }

  function ck(id, text, checked, hint) {
    return '<label class="ck"><input type="checkbox" id="' + id + '"' + (checked ? ' checked' : '') + '><span>' + text + (hint ? '<br><span class="help">' + hint + '</span>' : '') + '</span></label>';
  }
  function fld(id, text, value, opts) {
    opts = opts || {};
    return '<label class="f"><span>' + text + '</span><input class="inp" id="' + id + '" type="' + (opts.type || 'text') + '" value="' + esc(value) + '"' +
      (opts.ph ? ' placeholder="' + esc(opts.ph) + '"' : '') + (opts.min ? ' min="' + opts.min + '"' : '') + ' autocomplete="off" spellcheck="false">' +
      (opts.help ? '<small>' + opts.help + '</small>' : '') + '</label>';
  }

  function buildSettings() {
    var wiz = mode === 'wizard';
    var h = '<h2>' + (wiz ? 'Vamos configurar' : 'Configurações') + '</h2>';
    h += '<p class="lead">' + (wiz ? 'Leva uns dois minutos. Dá para mudar tudo depois no botão Configurações.' : 'As alterações valem assim que você salvar.') + '</p>';
    if (wiz) h += '<div class="steps" id="steps">' + new Array(STEPS + 1).join('<i></i>') + '</div>';

    h += '<div class="sec" data-step="0"><h3>1. Seus chats</h3>' +
      ck('f-tw-on', '<b>Ler o chat da Twitch</b>', cfg.twitch.enabled) +
      fld('f-tw-ch', 'Canal da Twitch', cfg.twitch.channel, { ph: 'seucanal ou twitch.tv/seucanal' }) +
      ck('f-kk-on', '<b>Ler o chat da Kick</b>', cfg.kick.enabled) +
      fld('f-kk-ch', 'Canal da Kick', cfg.kick.channel, { ph: 'seucanal ou kick.com/seucanal' }) +
      '<details><summary>Avançado: bot que responde no chat da Twitch e ID do chat da Kick</summary>' +
      ck('f-tw-reply', 'O bot responde no chat da Twitch', cfg.twitch.reply, 'Sem isso o app só lê o chat; os viewers não recebem confirmação.') +
      fld('f-tw-bot', 'Usuário do bot', cfg.twitch.botUser, { ph: 'pode ser a sua própria conta' }) +
      fld('f-tw-token', 'Token OAuth da Twitch', '', { type: 'password', ph: cfg.twitchTokenSet ? '(salvo) deixe em branco para manter' : 'cole o token aqui', help: 'Veja no README como gerar. Fica só neste computador.' }) +
      (cfg.twitchTokenSet ? ck('f-tw-token-clear', 'Remover o token salvo', false) : '') +
      fld('f-kk-room', 'ID do chat da Kick (opcional)', cfg.kick.chatroomId || '', { type: 'number', min: 0, help: 'Só preencha se o status da Kick mostrar erro ao buscar o ID.' }) +
      fld('f-kk-pkey', 'Chave do Pusher da Kick (opcional)', cfg.kick.pusherKey, { ph: 'padrão', help: 'Só mude se a Kick trocar a chave e o chat parar de conectar.' }) +
      fld('f-kk-pcl', 'Cluster do Pusher da Kick (opcional)', cfg.kick.pusherCluster, { ph: 'us2' }) +
      '</details></div>';

    h += '<div class="sec" data-step="1"><h3>2. Lista de músicas</h3>' +
      '<label class="f"><span>De onde ler as músicas</span><select class="inp" id="f-sg-mode">' +
      '<option value="json"' + (cfg.songs.mode === 'json' ? ' selected' : '') + '>Arquivo JSON exportado pelo jogo</option>' +
      '<option value="folder"' + (cfg.songs.mode === 'folder' ? ' selected' : '') + '>Pasta de músicas (lê os song.ini)</option></select></label>' +
      fld('f-sg-path', 'Caminho', cfg.songs.path, { ph: 'C:\\Users\\voce\\Documents\\songs.json', help: 'Copie o caminho completo do arquivo ou da pasta.' }) +
      '<div class="row"><button class="btn" id="btn-test" type="button">Testar leitura</button><span id="test-out" class="grow"></span></div></div>';

    h += '<div class="sec" data-step="2"><h3>3. Regras dos pedidos</h3>' +
      fld('f-ru-user', 'Máximo de pedidos por pessoa na fila', cfg.rules.maxPerUser, { type: 'number', min: 1 }) +
      fld('f-ru-max', 'Tamanho máximo da fila', cfg.rules.maxQueue, { type: 'number', min: 1 }) +
      ck('f-ru-dup', 'Bloquear música que já está na fila', cfg.rules.blockDuplicates) +
      ck('f-ru-prio', 'Subs, VIPs e moderadores furam a fila', cfg.rules.priorityRoles, 'Entram depois do último pedido prioritário. O streamer nunca tem limite.') + '</div>';

    h += '<div class="sec" data-step="3"><h3>4. Site público e overlay</h3>' +
      '<p class="help" style="margin-top:0">Opcional. Para os viewers verem a lista no GitHub Pages, informe o repositório e um token. Dá para configurar depois.</p>' +
      fld('f-st-repo', 'Repositório no GitHub', cfg.site.repo, { ph: 'usuario/hero-requests' }) +
      fld('f-st-token', 'Token do GitHub', '', { type: 'password', ph: cfg.siteTokenSet ? '(salvo) deixe em branco para manter' : 'github_pat_…', help: 'Token com permissão Contents: leitura e escrita só nesse repositório.' }) +
      (cfg.siteTokenSet ? ck('f-st-token-clear', 'Remover o token salvo', false) : '') +
      ck('f-st-auto', 'Publicar a fila automaticamente', cfg.site.autoPublish, 'Atualiza o site alguns segundos depois de cada mudança.') +
      fld('f-st-url', 'Endereço do site (opcional)', cfg.site.url, { ph: 'https://usuario.github.io/hero-requests/', help: 'Usado no comando !lista do chat.' }) +
      '<div class="row"><div class="grow">' + fld('f-ov-items', 'Músicas na fila do overlay', cfg.overlay.items, { type: 'number', min: 1 }) + '</div>' +
      '<div class="grow"><label class="f"><span>Cor de destaque</span><input class="inp" id="f-ov-accent" type="color" value="' + esc(cfg.overlay.accent) + '" style="padding:2px;height:36px"></label></div></div>' +
      (wiz ? '' : '<details><summary>Avançado</summary>' + fld('f-port', 'Porta do painel', cfg.port, { type: 'number', min: 1024, help: 'Reinicie o app depois de mudar.' }) + '</details>') + '</div>';

    h += '<div class="sec" data-step="4"><h3>Tudo pronto</h3>' +
      '<p>No OBS, adicione uma fonte <b>Navegador (Browser Source)</b> com este endereço:</p>' +
      '<p class="url card">' + esc(location.origin) + '/overlay</p>' +
      '<p class="help">Largura 1920, altura 1080. Fundo transparente. Para testar agora, use “Simular pedido” no painel.</p></div>';

    h += '<div class="nav" id="nav"></div><div id="set-err" class="alert" hidden style="margin-top:12px"></div>';
    $('#settings').innerHTML = h;
    $('#btn-test').addEventListener('click', testSongs);
    syncStep();
  }

  function syncStep() {
    var wiz = mode === 'wizard';
    document.querySelectorAll('#settings .sec').forEach(function (s) {
      var n = +s.dataset.step;
      s.hidden = wiz ? n !== step : n === 4;
    });
    if (wiz) document.querySelectorAll('#steps i').forEach(function (i, n) { i.classList.toggle('on', n <= step); });
    var nav = $('#nav');
    if (!wiz) {
      nav.innerHTML = '<button class="btn" id="n-cancel" type="button">Voltar ao painel</button>' +
        '<button class="btn p" id="n-save" type="button">Salvar</button>';
      $('#n-cancel').onclick = showPanel;
      $('#n-save').onclick = function () { save(false); };
    } else if (step < 4) {
      nav.innerHTML = (step > 0 ? '<button class="btn" id="n-back" type="button">Voltar</button>' : '<span></span>') +
        '<span>' + (step === 1 ? '<button class="btn" id="n-skip" type="button">Pular por enquanto</button> ' : '') +
        '<button class="btn p" id="n-next" type="button">' + (step === 3 ? 'Concluir' : 'Próximo') + '</button></span>';
      if (step > 0) $('#n-back').onclick = function () { step--; syncStep(); };
      if (step === 1) $('#n-skip').onclick = function () { step++; syncStep(); };
      $('#n-next').onclick = next;
    } else {
      nav.innerHTML = '<span></span><button class="btn p" id="n-go" type="button">Abrir o painel</button>';
      $('#n-go').onclick = showPanel;
    }
    window.scrollTo(0, 0);
  }

  function showErr(m) { var e = $('#set-err'); e.textContent = m || ''; e.hidden = !m; }

  async function next() {
    showErr('');
    if (step === 0) {
      var tw = $('#f-tw-on').checked && $('#f-tw-ch').value.trim();
      var kk = $('#f-kk-on').checked && $('#f-kk-ch').value.trim();
      if (!tw && !kk) return showErr('Informe pelo menos um canal (Twitch ou Kick) e deixe a opção marcada.');
    }
    if (step === 1) {
      if (!$('#f-sg-path').value.trim()) return showErr('Informe o caminho das músicas, ou use “Pular por enquanto”.');
      if (!(await testSongs())) return showErr('Não consegui ler as músicas nesse caminho. Corrija ou use “Pular por enquanto”.');
    }
    if (step === 3) { if (!(await save(true))) return; }
    step++; syncStep();
  }

  async function testSongs() {
    var out = $('#test-out'); out.className = 'grow'; out.textContent = 'Lendo…';
    try {
      var r = await api('/api/songs/test', 'POST', { mode: $('#f-sg-mode').value, path: $('#f-sg-path').value });
      out.className = 'grow test-ok';
      out.textContent = '✓ ' + r.count.toLocaleString('pt-BR') + ' músicas. Ex.: ' + r.sample.join(' · ');
      return true;
    } catch (e) { out.className = 'grow test-err'; out.textContent = '✗ ' + e.message; return false; }
  }

  function tokenValue(id, clearId) {
    var clr = $('#' + clearId);
    if (clr && clr.checked) return '__clear__';
    return $('#' + id).value;
  }

  function collect(finish) {
    return {
      configured: finish || cfg.configured || mode !== 'wizard',
      port: +($('#f-port') ? $('#f-port').value : cfg.port) || cfg.port,
      twitch: {
        enabled: $('#f-tw-on').checked, channel: $('#f-tw-ch').value,
        reply: $('#f-tw-reply').checked, botUser: $('#f-tw-bot').value,
        token: tokenValue('f-tw-token', 'f-tw-token-clear')
      },
      kick: { enabled: $('#f-kk-on').checked, channel: $('#f-kk-ch').value, chatroomId: +$('#f-kk-room').value || 0, pusherKey: $('#f-kk-pkey').value, pusherCluster: $('#f-kk-pcl').value },
      songs: { mode: $('#f-sg-mode').value, path: $('#f-sg-path').value },
      rules: {
        maxPerUser: +$('#f-ru-user').value, maxQueue: +$('#f-ru-max').value,
        blockDuplicates: $('#f-ru-dup').checked, priorityRoles: $('#f-ru-prio').checked
      },
      site: {
        repo: $('#f-st-repo').value, branch: cfg.site.branch,
        token: tokenValue('f-st-token', 'f-st-token-clear'),
        autoPublish: $('#f-st-auto').checked, url: $('#f-st-url').value
      },
      overlay: { items: +$('#f-ov-items').value, accent: $('#f-ov-accent').value }
    };
  }

  async function save(finish) {
    showErr('');
    try {
      var r = await api('/api/config', 'PUT', collect(finish));
      cfg = r.config;
      if (!finish) { toast(r.restartForPort ? 'Salvo. Reinicie o app para usar a nova porta.' : 'Configurações salvas'); showPanel(); }
      return true;
    } catch (e) { showErr(e.message); return false; }
  }

  connect();
})();
