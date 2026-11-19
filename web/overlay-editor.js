// Editor do overlay: formulário + pré-visualização ao vivo (iframe com dados de exemplo).
(function () {
  'use strict';
  var $ = function (s, el) { return (el || document).querySelector(s); };
  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  var FONTS = [
    ['"Segoe UI", system-ui, sans-serif', 'Segoe UI (padrão)'],
    ['Arial, Helvetica, sans-serif', 'Arial'],
    ['Verdana, Geneva, sans-serif', 'Verdana'],
    ['Tahoma, sans-serif', 'Tahoma'],
    ['"Trebuchet MS", sans-serif', 'Trebuchet MS'],
    ['Georgia, serif', 'Georgia'],
    ['"Times New Roman", serif', 'Times New Roman'],
    ['Impact, sans-serif', 'Impact'],
    ['Consolas, "Courier New", monospace', 'Consolas (monoespaçada)'],
    ['"Comic Sans MS", cursive', 'Comic Sans']
  ];
  var CUSTOM = '__custom';

  var SCHEMA = [
    { sec: 'Posição e tamanho', f: [
      { k: 'corner', t: 'select', l: 'Posição na tela', o: [['tr', 'Canto superior direito'], ['tl', 'Canto superior esquerdo'], ['br', 'Canto inferior direito'], ['bl', 'Canto inferior esquerdo']] },
      { k: 'margin', t: 'range', l: 'Margem da borda', min: 0, max: 200, u: 'px' },
      { k: 'width', t: 'range', l: 'Largura', min: 200, max: 900, u: 'px' },
      { k: 'items', t: 'range', l: 'Músicas visíveis na fila', min: 1, max: 12, u: '' }
    ] },
    { sec: 'Fonte', f: [
      { k: 'fontFamily', t: 'font', l: 'Fonte' },
      { k: 'fontSize', t: 'range', l: 'Tamanho do texto', min: 8, max: 48, u: 'px' },
      { k: 'titleSize', t: 'range', l: 'Tamanho do título da música atual', min: 10, max: 80, u: 'px' }
    ] },
    { sec: 'Cores', f: [
      { k: 'accent', t: 'color', l: 'Destaque' },
      { k: 'textColor', t: 'color', l: 'Texto' },
      { k: 'mutedColor', t: 'color', l: 'Texto secundário' },
      { k: 'bgColor', t: 'color', l: 'Fundo das caixas' },
      { k: 'borderColor', t: 'color', l: 'Borda' }
    ] },
    { sec: 'Caixas', f: [
      { k: 'bgOpacity', t: 'range', l: 'Opacidade do fundo', min: 0, max: 100, u: '%' },
      { k: 'borderOpacity', t: 'range', l: 'Opacidade da borda', min: 0, max: 100, u: '%' },
      { k: 'borderWidth', t: 'range', l: 'Espessura da borda', min: 0, max: 12, u: 'px' },
      { k: 'radius', t: 'range', l: 'Arredondamento', min: 0, max: 60, u: 'px' },
      { k: 'padding', t: 'range', l: 'Espaço interno', min: 0, max: 60, u: 'px' },
      { k: 'gap', t: 'range', l: 'Espaço entre caixas', min: 0, max: 60, u: 'px' },
      { k: 'blur', t: 'range', l: 'Desfoque do fundo (vidro)', min: 0, max: 40, u: 'px' },
      { k: 'accentBar', t: 'check', l: 'Barra colorida na música atual' }
    ] },
    { sec: 'Conteúdo', f: [
      { k: 'showNow', t: 'check', l: 'Mostrar a música tocando agora' },
      { k: 'showQueue', t: 'check', l: 'Mostrar a fila' },
      { k: 'showHint', t: 'check', l: 'Mostrar a dica de como pedir' },
      { k: 'showBadges', t: 'check', l: 'Mostrar o ícone da plataforma (T / K)' },
      { k: 'showRequester', t: 'check', l: 'Mostrar quem pediu' },
      { k: 'showLabels', t: 'check', l: 'Mostrar os títulos das caixas' },
      { k: 'nowLabel', t: 'text', l: 'Título da música atual', max: 40 },
      { k: 'queueLabel', t: 'text', l: 'Título da fila', max: 40 },
      { k: 'hintText', t: 'text', l: 'Texto da dica', max: 120 }
    ] }
  ];

  var PRESETS = {
    'Escuro (padrão)': {},
    'Vidro': { bgColor: '#000000', bgOpacity: 45, blur: 14, borderColor: '#ffffff', borderOpacity: 22, radius: 16, textColor: '#ffffff', mutedColor: '#d0d4de', accentBar: false },
    'Claro': { bgColor: '#ffffff', textColor: '#1b1a24', mutedColor: '#6a6880', borderColor: '#dcd9e6', accent: '#d9480f' },
    'Neon': { bgColor: '#0a0014', bgOpacity: 92, borderColor: '#ff00e6', borderWidth: 2, accent: '#00f0ff', textColor: '#ffffff', mutedColor: '#b48cff', radius: 4, fontFamily: 'Consolas, "Courier New", monospace' },
    'Sem caixas': { bgOpacity: 0, borderWidth: 0, padding: 4, accentBar: false, showHint: false, customCss: '.box{text-shadow:0 1px 4px #000,0 0 2px #000}' }
  };
  var APPEARANCE_KEEP = ['items', 'corner', 'margin', 'width'];

  var st = { defaults: null, saved: null, draft: null, dirty: false, root: null, frame: null, onClose: null, api: null, toast: null, leaveArmed: false };

  function fontKnown(v) { return FONTS.some(function (f) { return f[0] === v; }); }

  function fieldHtml(f, v) {
    var id = 'ov-' + f.k;
    if (f.t === 'range') {
      return '<label class="f"><span>' + f.l + ' <b class="mute" id="' + id + '-v">' + v + f.u + '</b></span>' +
        '<input type="range" id="' + id + '" data-k="' + f.k + '" min="' + f.min + '" max="' + f.max + '" value="' + v + '"></label>';
    }
    if (f.t === 'color') {
      return '<label class="f"><span>' + f.l + '</span><span class="row"><input type="color" id="' + id + '" data-k="' + f.k + '" value="' + esc(v) +
        '" style="width:48px;height:34px;padding:2px" class="inp"><code class="mute" id="' + id + '-v">' + esc(v) + '</code></span></label>';
    }
    if (f.t === 'check') {
      return '<label class="ck"><input type="checkbox" id="' + id + '" data-k="' + f.k + '"' + (v ? ' checked' : '') + '><span>' + f.l + '</span></label>';
    }
    if (f.t === 'select') {
      return '<label class="f"><span>' + f.l + '</span><select class="inp" id="' + id + '" data-k="' + f.k + '">' +
        f.o.map(function (o) { return '<option value="' + o[0] + '"' + (o[0] === v ? ' selected' : '') + '>' + o[1] + '</option>'; }).join('') + '</select></label>';
    }
    if (f.t === 'text') {
      return '<label class="f"><span>' + f.l + '</span><input class="inp" type="text" id="' + id + '" data-k="' + f.k + '" maxlength="' + f.max + '" value="' + esc(v) + '"></label>';
    }
    if (f.t === 'font') {
      var known = fontKnown(v);
      return '<label class="f"><span>' + f.l + '</span><select class="inp" id="ov-font-sel">' +
        FONTS.map(function (o) { return '<option value="' + esc(o[0]) + '"' + (o[0] === v ? ' selected' : '') + '>' + o[1] + '</option>'; }).join('') +
        '<option value="' + CUSTOM + '"' + (known ? '' : ' selected') + '>Outra fonte…</option></select></label>' +
        '<div id="ov-font-custom"' + (known ? ' hidden' : '') + '>' +
        '<label class="f"><span>Nome da fonte</span><input class="inp" type="text" id="ov-font-name" maxlength="120" value="' + esc(known ? '' : v) + '" placeholder="Ex.: Press Start 2P" spellcheck="false">' +
        '<small>Pode ser uma fonte instalada no seu PC ou do Google Fonts.</small></label>' +
        '<label class="ck"><input type="checkbox" id="ov-font-google" data-k="fontGoogle"' + (st.draft.fontGoogle ? ' checked' : '') + '><span>É do Google Fonts (precisa de internet no OBS)</span></label></div>';
    }
    return '';
  }

  function build() {
    var h = '<div class="ov-top"><div><h2>Personalizar overlay</h2><p class="lead" style="margin:0">As mudanças aparecem na pré-visualização na hora. Ao salvar, o OBS atualiza sozinho.</p></div>' +
      '<div class="row"><span id="ov-dirty" class="mute" hidden>alterações não salvas</span>' +
      '<button class="btn" id="ov-back" type="button">Voltar</button><button class="btn p" id="ov-save" type="button">Salvar</button></div></div>';
    h += '<div class="ov-wrap"><div class="ov-form">';
    h += '<div class="sec"><h3>Temas prontos</h3><div class="row"><select class="inp grow" id="ov-preset"><option value="">Escolher tema…</option>' +
      Object.keys(PRESETS).map(function (n) { return '<option>' + esc(n) + '</option>'; }).join('') +
      '</select><button class="btn" id="ov-reset" type="button">Restaurar padrão</button></div>' +
      '<div class="help" style="margin-top:6px">Um tema troca cores e estilo, mas mantém posição e tamanho.</div></div>';
    SCHEMA.forEach(function (s) {
      h += '<div class="sec"><h3>' + s.sec + '</h3>' + s.f.map(function (f) { return fieldHtml(f, st.draft[f.k]); }).join('') + '</div>';
    });
    h += '<div class="sec"><h3>Avançado</h3>' +
      '<label class="f"><span>CSS personalizado</span><textarea class="inp" id="ov-css" rows="6" spellcheck="false" placeholder=".box { box-shadow: 0 4px 20px #0008; }" style="font-family:ui-monospace,Consolas,monospace;font-size:12px">' + esc(st.draft.customCss) + '</textarea>' +
      '<small>Para quem quer ir além: classes <code>.box</code>, <code>.now</code>, <code>.lbl</code>, <code>.t</code>, <code>.row</code>, <code>.nm</code>, <code>.by</code>, <code>.hint</code>.</small></label>' +
      '<label class="f"><span>Exportar / importar tema</span><textarea class="inp" id="ov-json" rows="3" spellcheck="false" style="font-family:ui-monospace,Consolas,monospace;font-size:12px"></textarea></label>' +
      '<div class="row"><button class="btn sm" id="ov-export" type="button">Exportar tema atual</button><button class="btn sm" id="ov-import" type="button">Aplicar tema colado</button></div></div>';
    h += '</div><div class="ov-side"><div class="row" style="margin-bottom:8px"><span class="h grow">Pré-visualização (1920×1080)</span>' +
      '<select class="inp" id="ov-bg" style="width:auto"><option value="game">Fundo: jogo</option><option value="dark">Fundo: escuro</option><option value="light">Fundo: claro</option><option value="check">Fundo: xadrez</option></select></div>' +
      '<div class="pv game" id="pv"><iframe id="pv-frame" src="/overlay?preview=1" title="Pré-visualização do overlay"></iframe></div>' +
      '<p class="help">No OBS use <code>http://127.0.0.1:7777/overlay</code>. A pré-visualização usa pedidos de exemplo.</p></div></div>';
    st.root.innerHTML = h;
    st.frame = $('#pv-frame', st.root);
    bind();
    rescale();
  }

  function pushPreview() {
    if (st.frame && st.frame.contentWindow) {
      st.frame.contentWindow.postMessage({ type: 'overlay', cfg: st.draft }, location.origin);
    }
  }
  function setDirty(d) {
    st.dirty = d; st.leaveArmed = false;
    var el = $('#ov-dirty', st.root); if (el) el.hidden = !d;
  }
  function set(k, v) { st.draft[k] = v; setDirty(true); pushPreview(); }

  function rescale() {
    var pv = $('#pv', st.root); if (!pv) return;
    var s = pv.clientWidth / 1920;
    st.frame.style.transform = 'scale(' + s + ')';
    pv.style.height = Math.round(1080 * s) + 'px';
  }
  window.addEventListener('resize', function () { if (st.root && !st.root.hidden) rescale(); });

  function bind() {
    st.root.addEventListener('input', function (e) {
      var t = e.target, k = t.dataset && t.dataset.k;
      if (t.id === 'ov-css') return set('customCss', t.value);
      if (t.id === 'ov-font-name') { set('fontFamily', t.value.trim() || st.defaults.fontFamily); return; }
      if (!k) return;
      var v;
      if (t.type === 'checkbox') v = t.checked;
      else if (t.type === 'range') { v = +t.value; var lb = $('#ov-' + k + '-v', st.root); if (lb) lb.textContent = v + (t.dataset.u || unit(k)); }
      else if (t.type === 'color') { v = t.value; var cb = $('#ov-' + k + '-v', st.root); if (cb) cb.textContent = v; }
      else v = t.value;
      set(k, v);
    });
    st.root.addEventListener('change', function (e) {
      var t = e.target;
      if (t.id === 'ov-font-sel') {
        var custom = t.value === CUSTOM;
        $('#ov-font-custom', st.root).hidden = !custom;
        if (custom) { var n = $('#ov-font-name', st.root).value.trim(); set('fontFamily', n || st.defaults.fontFamily); }
        else { set('fontFamily', t.value); st.draft.fontGoogle = false; $('#ov-font-google', st.root).checked = false; }
      } else if (t.id === 'ov-preset') {
        if (t.value) applyPreset(t.value);
        t.value = '';
      } else if (t.id === 'ov-bg') {
        $('#pv', st.root).className = 'pv ' + t.value;
      }
    });
    $('#pv-frame', st.root).addEventListener('load', pushPreview);
    $('#ov-save', st.root).onclick = save;
    $('#ov-back', st.root).onclick = back;
    $('#ov-reset', st.root).onclick = function () { st.draft = Object.assign({}, st.defaults); rebuild(true); };
    $('#ov-export', st.root).onclick = function () { $('#ov-json', st.root).value = JSON.stringify(st.draft); $('#ov-json', st.root).select(); };
    $('#ov-import', st.root).onclick = function () {
      try {
        var o = JSON.parse($('#ov-json', st.root).value);
        if (!o || typeof o !== 'object') throw 0;
        st.draft = Object.assign({}, st.defaults, o); rebuild(true);
      } catch (_) { st.toast('Tema inválido: cole o texto exportado.'); }
    };
    // o iframe avisa quando está pronto
    window.addEventListener('message', function (e) {
      if (e.origin === location.origin && e.data && e.data.type === 'overlay-ready' && st.root && !st.root.hidden) pushPreview();
    });
  }

  function unit(k) { return ({ bgOpacity: '%', borderOpacity: '%' })[k] || (k === 'items' ? '' : 'px'); }

  function applyPreset(name) {
    var d = Object.assign({}, st.defaults, PRESETS[name]);
    APPEARANCE_KEEP.forEach(function (k) { d[k] = st.draft[k]; });
    st.draft = d; rebuild(true);
  }
  function rebuild(dirty) {
    var scroll = window.scrollY;
    build(); setDirty(!!dirty); pushPreview(); window.scrollTo(0, scroll);
  }

  async function save() {
    try {
      var saved = await st.api('/api/overlay', 'PUT', st.draft);
      st.saved = saved; st.draft = Object.assign({}, saved);
      rebuild(false);
      st.toast('Overlay salvo. O OBS já atualizou.');
    } catch (e) { st.toast(e.message); }
  }
  function back() {
    if (st.dirty && !st.leaveArmed) {
      st.leaveArmed = true; st.toast('Há alterações não salvas. Clique em Voltar de novo para descartar.'); return;
    }
    st.root.hidden = true; st.root.innerHTML = '';
    st.onClose();
  }

  window.OverlayEditor = {
    open: async function (root, api, toast, onClose) {
      st.root = root; st.api = api; st.toast = toast; st.onClose = onClose;
      try {
        st.defaults = await api('/api/overlay/defaults', 'GET');
        var cfg = await api('/api/config', 'GET');
        st.saved = cfg.overlay;
      } catch (e) { toast(e.message); return; }
      st.draft = Object.assign({}, st.saved);
      root.hidden = false;
      build(); setDirty(false);
    }
  };
})();
