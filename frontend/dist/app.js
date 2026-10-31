'use strict';

// Browser preview: stand-in for the Go bindings so the UI can be checked without the app.
if (!window.go) {
  const C = (id, hw) => ({ id, ok: true, hw: !!hw });
  const F = (id, ext, kind, codecs, lossless) => ({ id, ext, kind, lossless: !!lossless, ok: true, codecs: codecs || [] });
  const demo = {
    settings: { lang: 'ru', theme: 'dark', accent: 'clay', kind: 'video', format: 'mp4', codecs: {}, quality: 'mid', res: 0, hw: true, outMode: 'same', outDir: '', parallel: 1, overwrite: false, lowPriority: true, customExt: '', customArgs: '', ffmpeg: '' },
    running: false, version: 'preview',
    ffmpeg: { path: 'C:\\ffmpeg\\bin\\ffmpeg.exe', version: '9.0.2', hw: 'NVENC' },
    formats: [
      F('mp4', 'mp4', 'video', [C('h264', 1), C('h265', 1), C('av1'), C('copy')]), F('mkv', 'mkv', 'video', [C('h264', 1), C('h265', 1), C('av1'), C('copy')]),
      F('mov', 'mov', 'video', [C('h264', 1), C('h265', 1), C('prores'), C('copy')]), F('webm', 'webm', 'video', [C('vp9'), C('av1')]),
      F('avi', 'avi', 'video', [C('mpeg4')]), F('m4v', 'm4v', 'video', [C('h264', 1)]), F('wmv', 'wmv', 'video', [C('wmv2')]),
      F('flv', 'flv', 'video', [C('h264', 1)]), F('ts', 'ts', 'video', [C('h264', 1), C('h265', 1), C('copy')]), F('mpg', 'mpg', 'video', [C('mpeg2')]),
      F('ogv', 'ogv', 'video', [C('theora')]), F('gif', 'gif', 'video', [C('gif')]),
      F('mp3', 'mp3', 'audio'), F('m4a', 'm4a', 'audio'), F('aac', 'aac', 'audio'), F('wav', 'wav', 'audio', [], 1), F('flac', 'flac', 'audio', [], 1),
      F('ogg', 'ogg', 'audio'), F('opus', 'opus', 'audio'), F('wma', 'wma', 'audio'), F('aiff', 'aiff', 'audio', [], 1), F('ac3', 'ac3', 'audio'), F('alac', 'm4a', 'audio', [], 1),
      F('png', 'png', 'image', [], 1), F('jpg', 'jpg', 'image'), F('webp', 'webp', 'image'), F('avif', 'avif', 'image'), F('bmp', 'bmp', 'image', [], 1), F('tiff', 'tiff', 'image', [], 1), F('ico', 'ico', 'image', [], 1),
    ],
    jobs: [
      { id: '1', path: 'D:\\Video\\Отпуск 2026 — море.mkv', name: 'Отпуск 2026 — море.mkv', size: 2.1e9, kind: 'video', summary: '3840×2160 · hevc · eac3 · 14:32', duration: 872, status: 'done', progress: 1, out: 'D:\\Video\\Отпуск 2026 — море.mp4', outSize: 640e6, took: 212 },
      { id: '2', path: 'D:\\Video\\screen-record.mov', name: 'screen-record.mov', size: 380e6, kind: 'video', summary: '2560×1440 · prores · aac · 3:05', duration: 185, status: 'running', progress: 0.42, speed: 2.4, eta: 44 },
      { id: '3', path: 'D:\\Music\\track01.flac', name: 'track01.flac', size: 31e6, kind: 'audio', summary: 'flac · 44.1 kHz · 4:12', duration: 252, status: 'ready', progress: 0 },
      { id: '4', path: 'D:\\Pics\\cover.webp', name: 'cover.webp', size: 420e3, kind: 'image', summary: '1600×1600 · webp', duration: 0, status: 'ready', progress: 0 },
      { id: '5', path: 'D:\\Downloads\\notes.txt', name: 'notes.txt', size: 2e3, status: 'error', progress: 0, error: 'e.notMedia' },
    ],
  };
  const sp = new URLSearchParams(location.search);
  if (sp.get('lang')) demo.settings.lang = sp.get('lang');
  if (sp.get('theme')) demo.settings.theme = sp.get('theme');
  if (sp.has('empty')) demo.jobs = [];
  window.__shot = sp.get('shot');
  const evl = {};
  const fire = (name, data) => (evl[name] || []).forEach(f => f(data));
  const emit = () => fire('state', JSON.parse(JSON.stringify(demo)));
  window.runtime = { EventsOn: (name, f) => (evl[name] = evl[name] || []).push(f) };
  setInterval(() => {
    const r = demo.jobs.find(j => j.status === 'running');
    if (!r) return;
    r.progress = Math.min(1, r.progress + 0.01); r.eta = Math.max(0, (1 - r.progress) * r.duration / r.speed);
    if (r.progress >= 1 && demo.running) {
      Object.assign(r, { status: 'done', out: r.path.replace(/\.\w+$/, '.' + demo.settings.format), outSize: r.size / 3, took: 77 });
      const next = demo.jobs.find(j => j.status === 'ready');
      if (next) next.status = 'running', next.speed = 3.1; else { demo.running = false; fire('finished', { done: 2, failed: 0 }); }
    }
    emit();
  }, 300);
  window.go = { main: { App: {
    GetState: async () => JSON.parse(JSON.stringify(demo)),
    SaveSettings: async s => { demo.settings = s; emit(); },
    Start: async () => { demo.running = true; const j = demo.jobs.find(j => j.status === 'ready'); if (j && !demo.jobs.some(j => j.status === 'running')) j.status = 'running', j.speed = 2; emit(); },
    Stop: async () => { demo.running = false; demo.jobs.forEach(j => { if (j.status === 'running') j.status = 'ready', j.progress = 0; }); emit(); },
    Cancel: async id => { const j = demo.jobs.find(j => j.id === id); j.status = 'canceled'; emit(); },
    Retry: async id => { const j = demo.jobs.find(j => j.id === id); j.status = 'ready'; j.error = ''; emit(); },
    Remove: async id => { demo.jobs = demo.jobs.filter(j => j.id !== id); emit(); },
    ClearFinished: async () => { demo.jobs = demo.jobs.filter(j => ['ready', 'running', 'probing'].includes(j.status)); emit(); },
    ClearAll: async () => { demo.jobs = demo.jobs.filter(j => j.status === 'running'); emit(); },
    AddFiles: async () => 0, AddPaths: async () => 0,
    GetJobLog: async id => ({ cmd: id === '5' ? '' : 'ffmpeg -hide_banner -nostdin -y -v warning -progress pipe:1 -nostats -i "D:\\Video\\screen-record.mov" -c:v h264_nvenc -preset p5 -rc vbr -cq 24 -b:v 0 -pix_fmt yuv420p -vf scale=trunc(iw/2)*2:trunc(ih/2)*2 -c:a aac -b:a 192k -sn -dn -movflags +faststart "D:\\Video\\screen-record.mp4"', lines: id === '2' ? ['[aac @ 0000020] Too many bits 8832 > 6144 per frame requested, clamping to max'] : [] }),
    PickOutDir: async () => 'D:\\Converted', PickFFmpeg: async () => '',
    Reveal: async () => {}, OpenPath: async () => {},
    Minimise: async () => {}, ToggleMaximise: async () => {}, Quit: async () => {}, SetWindowDark: async () => {}, SaveWindowSize: async () => {},
  } } };
}

const api = window.go.main.App;
const $ = s => document.querySelector(s);
const $$ = s => document.querySelectorAll(s);
let state = null;
let field = null;
let toastTimer = 0;
const openJobs = new Set();

// Accent presets: [fill, accent text on the light theme], the same set as outbox.
const ACCENTS = {
  clay: ['#bb7463', '#94503f'],
  pink: ['#c97a98', '#9a4a68'],
  purple: ['#9a88c9', '#5f4d96'],
  blue: ['#7f9cc4', '#3d5f8c'],
  cyan: ['#6fb0b5', '#2f6e73'],
  green: ['#86a97a', '#4b6b40'],
  orange: ['#c9925a', '#8a5a24'],
  red: ['#c96a5e', '#9a3a2f'],
};
const CODEC_LABEL = { h264: 'H.264', h265: 'H.265', av1: 'AV1', vp9: 'VP9', prores: 'ProRes', mpeg4: 'MPEG-4', wmv2: 'WMV2', mpeg2: 'MPEG-2', theora: 'Theora', gif: 'GIF' };
const darkQuery = window.matchMedia('(prefers-color-scheme: dark)');
let appliedTheme = '';

function applyTheme() {
  if (!state) return;
  const st = state.settings;
  const theme = st.theme === 'system' ? (darkQuery.matches ? 'dark' : 'light') : st.theme;
  const root = document.documentElement;
  const [fill, lightInk] = ACCENTS[st.accent] || ACCENTS.clay;
  const changed = root.dataset.theme !== theme || root.style.getPropertyValue('--accent') !== fill;
  root.dataset.theme = theme;
  root.style.setProperty('--accent', fill);
  root.style.setProperty('--accent-ink', theme === 'dark' ? fill : lightInk);
  if (changed && field) field.recolor();
  if (theme !== appliedTheme) {
    appliedTheme = theme;
    api.SetWindowDark(theme === 'dark');
  }
}
darkQuery.addEventListener('change', applyTheme);

const lang = () => (state && I18N[state.settings.lang]) ? state.settings.lang : 'ru';
function t(key, ...args) {
  const v = I18N[lang()][key] ?? I18N.ru[key] ?? key;
  return typeof v === 'function' ? v(...args) : v;
}
const errText = e => (e && e.startsWith('e.')) ? t(e) : e;

function toast(msg, kind = 'info') {
  const el = $('#toast');
  el.textContent = msg;
  el.className = 'toast show ' + kind;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove('show'), 3200);
}

const esc = s => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
function size(b) {
  if (!b) return '';
  const u = lang() === 'ru' ? ['Б', 'КБ', 'МБ', 'ГБ'] : ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  while (b >= 1000 && i < u.length - 1) { b /= 1000; i++; }
  return (i ? b.toFixed(b < 10 ? 1 : 0) : b) + ' ' + u[i];
}
function clock(sec) {
  sec = Math.max(0, Math.round(sec));
  const h = Math.floor(sec / 3600), m = Math.floor(sec / 60) % 60, s = sec % 60;
  return h ? `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}` : `${m}:${String(s).padStart(2, '0')}`;
}
const BAR = 22;
function asciiBar(p) {
  if (p < 0) {
    // unknown length: a short block sweeps along the track
    const pos = Math.floor(Date.now() / 90) % (BAR + 4) - 4;
    let s = '';
    for (let i = 0; i < BAR; i++) s += i >= pos && i < pos + 4 ? '<span class="scan">=</span>' : '<span class="r">·</span>';
    return `<span class="bar-ascii">[${s}]</span>`;
  }
  const n = Math.round(p * BAR);
  return `<span class="bar-ascii">[<span class="f">${'='.repeat(n)}</span><span class="r">${'·'.repeat(BAR - n)}</span>]</span>`;
}

// ---- settings ----

// Settings changed here win over the state pushes that are already on their
// way from Go with the old values, until Go has taken the save.
let savesInFlight = 0;
function save(patch) {
  Object.assign(state.settings, patch);
  savesInFlight++;
  api.SaveSettings(JSON.parse(JSON.stringify(state.settings)))
    .catch(e => toast(errText(String(e)), 'err'))
    .finally(() => { savesInFlight--; });
  render(state);
}
let saveTimer = 0;
function saveSoon(patch) {
  Object.assign(state.settings, patch);
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => { saveTimer = 0; save({}); }, 350);
}

function setSeg(sel, value) {
  $$(sel + ' button').forEach(b => b.classList.toggle('on', b.dataset.value === String(value)));
}
function bindSeg(sel, fn) {
  $(sel).addEventListener('click', e => {
    const b = e.target.closest('button');
    if (b && !b.disabled) fn(b.dataset.value);
  });
}

const fmtOf = id => (state.formats || []).find(f => f.id === id);
function currentCodec(f) {
  if (!f || !(f.codecs || []).length) return '';
  const c = state.settings.codecs[f.id];
  return f.codecs.some(x => x.id === c && x.ok) ? c : (f.codecs.find(x => x.ok) || f.codecs[0]).id;
}

// ---- render ----

function renderTexts() {
  document.documentElement.lang = lang();
  $$('[data-t]').forEach(el => { el.textContent = t(el.dataset.t); });
}

function renderPanel() {
  const s = state.settings, tools = state.ffmpeg;
  setSeg('#kind', s.kind);
  const list = (state.formats || []).filter(f => f.kind === s.kind);
  $('#formats').innerHTML = list.map(f =>
    `<button class="chip${f.id === s.format ? ' on' : ''}" data-id="${f.id}"${f.ok ? '' : ' disabled'}>${f.id}</button>`).join('');
  const own = !!(s.customArgs.trim() || s.customExt);
  $('#formats').style.opacity = s.customExt ? .45 : '';

  const f = fmtOf(s.format);
  const codec = currentCodec(f);
  const codecs = (f && f.codecs) || [];
  $('#b-codec').classList.toggle('off', own || codecs.length < 2);
  $('#codec').innerHTML = codecs.map(c =>
    `<button data-value="${c.id}"${c.ok ? '' : ' disabled'}>${c.id === 'copy' ? esc(t('codecCopy')) : CODEC_LABEL[c.id] || c.id}</button>`).join('');
  setSeg('#codec', codec);

  const copy = codec === 'copy';
  $('#b-quality').classList.toggle('off', own || copy || !f || f.lossless);
  setSeg('#quality', s.quality);
  $('#b-res').classList.toggle('off', own || copy || !f || f.kind === 'audio' || f.id === 'ico');
  setSeg('#res', s.res);

  const cinfo = codecs.find(c => c.id === codec);
  const hwShown = !own && f && f.kind === 'video' && tools && tools.hw && codecs.some(c => c.hw);
  $('#b-hw').classList.toggle('off', !hwShown);
  if (hwShown) {
    const usable = cinfo && cinfo.hw;
    $('#hw').checked = s.hw && usable;
    $('#hw').disabled = !usable;
    $('#hw-name').textContent = usable ? tools.hw : t('gpuOff');
  }

  setSeg('#outmode', s.outMode);
  const dir = s.outMode === 'dir';
  $('#outdir').classList.toggle('off', !dir);
  const pt = $('#outdir-text');
  pt.textContent = s.outDir || t('outPick');
  pt.classList.toggle('none', !s.outDir);
  pt.title = s.outDir;

  $('#btn-custom').classList.toggle('set', own);
  if (document.activeElement !== $('#c-ext')) $('#c-ext').value = s.customExt;
  if (document.activeElement !== $('#c-args')) $('#c-args').value = s.customArgs;
}

function renderGo() {
  const jobs = state.jobs;
  const waiting = jobs.filter(j => j.status === 'ready' || j.status === 'probing').length;
  const finished = jobs.filter(j => ['done', 'error', 'canceled'].includes(j.status)).length;
  const btn = $('#btn-go');
  btn.classList.toggle('stop', state.running);
  btn.textContent = state.running ? t('stop') : (waiting || !finished ? t('go') : t('goAgain'));
  btn.disabled = !state.running && (!state.ffmpeg || !state.ffmpeg.path || (!waiting && !finished));

  let note = '';
  if (!state.ffmpeg) note = t('noteWait');
  else if (state.running) {
    const active = jobs.filter(j => ['running', 'ready', 'probing', 'done', 'error'].includes(j.status));
    const done = jobs.filter(j => j.status === 'done').length;
    const eta = jobs.filter(j => j.status === 'running').reduce((m, j) => Math.max(m, j.eta || 0), 0)
      + jobs.filter(j => j.status === 'ready').reduce((sum, j) => sum + (j.duration || 0) / 3, 0) / state.settings.parallel;
    note = t('noteDone', done, active.length) + (eta > 1 ? ' · ' + t('noteEta', clock(eta)) : '');
  } else if (jobs.length) {
    const failed = jobs.filter(j => j.status === 'error').length;
    note = t('noteFiles', jobs.length) + (failed ? ' · ' + t('noteFailed', failed) : '');
  }
  $('#go-note').textContent = note;
}

function jobSub(j) {
  switch (j.status) {
    case 'probing': return esc(t('sProbing'));
    case 'running': {
      const p = j.progress;
      return asciiBar(p) + (p >= 0 ? ` ${Math.floor(p * 100)}%` : '') + (j.speed > 0 ? ` · ${j.speed.toFixed(1)}x` : '');
    }
    case 'done': return `<span class="arrow">-></span> ${esc(j.out.split('\\').pop())}`;
    case 'error': return `<span class="err">${esc(errText(j.error))}</span>`;
    case 'canceled': return esc(t('sCanceled'));
  }
  return esc(j.summary || '');
}
function jobMeta(j) {
  switch (j.status) {
    case 'running': return j.eta > 0 ? clock(j.eta) : '';
    case 'done': {
      const pct = j.size ? Math.round(j.outSize / j.size * 100) : 0;
      return size(j.outSize) + (pct ? ` · ${pct}%` : '');
    }
    case 'ready': return size(j.size);
  }
  return '';
}
function jobAction(j) {
  switch (j.status) {
    case 'running': return ['x', 'cancel', 'del'];
    case 'error': case 'canceled': return ['↻', 'retry', ''];
    case 'done': return ['/', 'reveal', ''];
  }
  return ['x', 'remove', 'del'];
}

function jobDetail(j) {
  const b = (act, key) => `<button class="gl-btn gl-btn-ghost gl-btn-sm" data-act="${act}">${esc(t(key))}</button>`;
  let acts = '';
  if (j.status === 'done') acts += b('open', 'open') + b('reveal', 'reveal');
  if (j.status === 'error' || j.status === 'canceled' || j.status === 'done') acts += b('retry', 'retry');
  if (j.status === 'running') acts += b('cancel', 'cancel');
  acts += b('copy', 'copyCmd') + b('remove', 'remove');
  return `<div class="detail-actions">${acts}</div><div class="code log"></div>`;
}

function fillLog(el, id) {
  api.GetJobLog(id).then(l => {
    const box = el.querySelector('.log');
    if (!box) return;
    const lines = [];
    if (l.cmd) lines.push(`<span class="cmd">${esc(l.cmd)}</span>`);
    lines.push(...(l.lines || []).map(esc));
    if (!l.cmd && !(l.lines || []).length) lines.push(esc(t('noLog')));
    box.innerHTML = lines.join('\n');
    el.dataset.cmd = l.cmd || '';
  });
}

function renderQueue() {
  const q = $('#queue');
  const jobs = state.jobs;
  document.body.classList.toggle('empty', !jobs.length);
  $('#q-count').textContent = jobs.length || '';
  const seen = new Set();
  let prev = null;
  for (const j of jobs) {
    seen.add(j.id);
    let el = q.querySelector(`.job[data-id="${j.id}"]`);
    if (!el) {
      el = document.createElement('div');
      el.className = 'job';
      el.dataset.id = j.id;
      el.innerHTML = '<div class="job-row"><span class="job-glyph" aria-hidden="true"></span><div class="job-text"><div class="job-name"></div><div class="job-sub"></div></div><span class="job-meta"></span><button class="gbtn"></button></div><div class="job-detail"></div>';
      el.querySelector('.job-name').textContent = j.name;
      el.title = j.path;
    }
    const statusChanged = el.dataset.s !== j.status;
    el.dataset.s = j.status;
    const sub = jobSub(j);
    const subEl = el.querySelector('.job-sub');
    if (subEl.innerHTML !== sub) subEl.innerHTML = sub;
    el.querySelector('.job-meta').textContent = jobMeta(j);
    const [glyph, act, cls] = jobAction(j);
    const gb = el.querySelector('.gbtn');
    gb.textContent = glyph; gb.dataset.act = act; gb.className = 'gbtn ' + cls; gb.title = t(act); gb.setAttribute('aria-label', t(act));
    const open = openJobs.has(j.id);
    el.classList.toggle('open', open);
    if (open && (statusChanged || !el.querySelector('.log'))) {
      el.querySelector('.job-detail').innerHTML = jobDetail(j);
      fillLog(el, j.id);
    }
    // keep DOM order equal to the queue order without touching nodes that are in place
    const want = prev ? prev.nextSibling : q.firstChild;
    if (want !== el) q.insertBefore(el, want);
    prev = el;
  }
  q.querySelectorAll('.job').forEach(el => { if (!seen.has(el.dataset.id)) { el.remove(); openJobs.delete(el.dataset.id); } });
}

function renderSettings() {
  const s = state.settings;
  setSeg('#set-lang', s.lang);
  setSeg('#set-theme', s.theme);
  setSeg('#set-parallel', s.parallel);
  $$('#set-accent .swatch').forEach(b => b.classList.toggle('on', b.dataset.value === s.accent));
  $('#set-lowprio').checked = s.lowPriority;
  $('#set-overwrite').checked = s.overwrite;
  const tl = state.ffmpeg;
  const ok = tl && tl.path;
  $('#ff-version').textContent = !tl ? t('ffLoading') : ok ? `ffmpeg ${tl.version}${tl.hw ? ' · ' + tl.hw : ''}` : t('ffMissing');
  const p = $('#ff-path');
  p.textContent = ok ? tl.path : tl ? t('ffMissingDesc') : '';
  p.classList.toggle('err', !!tl && !ok);
  $('#ff-auto').disabled = !s.ffmpeg;
  $('#version').textContent = 'recast ' + state.version;

  const foot = $('#ff-status');
  foot.classList.toggle('err', !!tl && !ok);
  foot.innerHTML = !tl ? esc(t('ffLoading')) : ok ? `ffmpeg ${esc(tl.version)}${tl.hw ? ` · <b>${esc(tl.hw.toLowerCase())}</b>` : ''}` : esc(t('ffMissing'));
}

let lastLang = '';
function render(st) {
  state = st;
  applyTheme();
  if (lastLang !== lang()) { lastLang = lang(); renderTexts(); }
  renderPanel();
  renderQueue();
  renderGo();
  renderSettings();
}

// ---- pages ----

function showPage(name) {
  $$('.page').forEach(p => p.classList.toggle('on', p.id === 'page-' + name));
  $$('.gl-nav-item').forEach(b => b.classList.toggle('on', b.dataset.page === name));
}
$('#nav').addEventListener('click', e => {
  const b = e.target.closest('.gl-nav-item');
  if (b) showPage(b.dataset.page);
});

// ---- events ----

bindSeg('#kind', kind => {
  const s = state.settings;
  if (kind === s.kind) return;
  // switching the tab picks the first format there unless the current one belongs to it
  const cur = fmtOf(s.format);
  const first = (state.formats || []).find(f => f.kind === kind && f.ok);
  save({ kind, format: cur && cur.kind === kind ? s.format : (first ? first.id : s.format) });
});
$('#formats').addEventListener('click', e => {
  const b = e.target.closest('.chip');
  if (b && !b.disabled) save({ format: b.dataset.id, customExt: '' });
});
bindSeg('#codec', c => save({ codecs: { ...state.settings.codecs, [state.settings.format]: c } }));
bindSeg('#quality', q => save({ quality: q }));
bindSeg('#res', r => save({ res: +r }));
$('#hw').addEventListener('change', e => save({ hw: e.target.checked }));
bindSeg('#outmode', async m => {
  if (m === 'dir' && !state.settings.outDir) {
    const d = await api.PickOutDir();
    if (!d) return;
    save({ outMode: m, outDir: d });
    return;
  }
  save({ outMode: m });
});
$('#outdir').addEventListener('click', async () => {
  const d = await api.PickOutDir();
  if (d) save({ outDir: d });
});
$('#btn-custom').addEventListener('click', () => {
  $('#btn-custom').classList.toggle('open');
  $('#custom').classList.toggle('open');
});
$('#c-ext').addEventListener('input', e => saveSoon({ customExt: e.target.value.replace(/^\.+/, '').trim() }));
$('#c-args').addEventListener('input', e => saveSoon({ customArgs: e.target.value }));
$('#c-ext').addEventListener('blur', () => { clearTimeout(saveTimer); save({}); });
$('#c-args').addEventListener('blur', () => { clearTimeout(saveTimer); save({}); });

$('#btn-go').addEventListener('click', () => (state.running ? api.Stop() : api.Start()));
const addFiles = () => api.AddFiles().then(n => { if (n) toast(t('added', n)); });
$('#btn-add').addEventListener('click', addFiles);
$('#btn-pick').addEventListener('click', addFiles);
$('#btn-clear-done').addEventListener('click', () => api.ClearFinished());
$('#btn-clear').addEventListener('click', () => api.ClearAll());

$('#queue').addEventListener('click', e => {
  const el = e.target.closest('.job');
  if (!el) return;
  const id = el.dataset.id;
  const j = state.jobs.find(x => x.id === id);
  const act = e.target.closest('[data-act]');
  if (act) {
    switch (act.dataset.act) {
      case 'cancel': api.Cancel(id); break;
      case 'retry': api.Retry(id); break;
      case 'remove': api.Remove(id); break;
      case 'reveal': if (j) api.Reveal(j.out); break;
      case 'open': if (j) api.OpenPath(j.out); break;
      case 'copy':
        navigator.clipboard.writeText(el.dataset.cmd || '').then(() => toast(t('copied')));
        break;
    }
    return;
  }
  if (e.target.closest('.job-detail')) return;
  if (openJobs.has(id)) openJobs.delete(id); else openJobs.add(id);
  el.querySelector('.job-detail').innerHTML = '';
  renderQueue();
});
// the open log of a running file follows ffmpeg's output
setInterval(() => {
  if (!state) return;
  for (const id of openJobs) {
    const j = state.jobs.find(x => x.id === id);
    const el = document.querySelector(`.job[data-id="${id}"]`);
    if (j && el && j.status === 'running') fillLog(el, id);
  }
}, 1000);
// the sweep of files whose length is unknown
setInterval(() => {
  if (!state || !state.jobs.some(j => j.status === 'running' && j.progress < 0)) return;
  renderQueue();
}, 90);

bindSeg('#set-lang', v => save({ lang: v }));
bindSeg('#set-theme', v => save({ theme: v }));
bindSeg('#set-parallel', v => save({ parallel: +v }));
$('#set-accent').innerHTML = Object.entries(ACCENTS)
  .map(([k, [c]]) => `<button class="swatch" data-value="${k}" style="--c:${c}" aria-label="${k}"></button>`).join('');
$('#set-accent').addEventListener('click', e => {
  const b = e.target.closest('.swatch');
  if (b) save({ accent: b.dataset.value });
});
$('#set-lowprio').addEventListener('change', e => save({ lowPriority: e.target.checked }));
$('#set-overwrite').addEventListener('change', e => save({ overwrite: e.target.checked }));
$('#ff-pick').addEventListener('click', async () => {
  const p = await api.PickFFmpeg();
  if (p) save({ ffmpeg: p });
});
$('#ff-auto').addEventListener('click', () => save({ ffmpeg: '' }));

$('#btn-min').addEventListener('click', () => api.Minimise());
$('#btn-max').addEventListener('click', () => api.ToggleMaximise());
$('#btn-close').addEventListener('click', () => api.Quit());
$('.bar').addEventListener('dblclick', e => { if (!e.target.closest('button')) api.ToggleMaximise(); });

// Dropped files: Wails' OnFileDrop must be registered from JS — it installs the
// drop handler that cancels the webview's own drop (which would open the file)
// and hands the real paths over. Without it WebView2 navigates to the file.
if (window.runtime.OnFileDrop) {
  window.runtime.OnFileDrop((x, y, paths) => {
    api.AddPaths(paths).then(n => toast(n ? t('added', n) : t('nothingAdded'), n ? 'info' : 'warn'));
  }, false);
}
let dragDepth = 0;
const hasFiles = e => e.dataTransfer && Array.from(e.dataTransfer.types || []).includes('Files');
const endDrag = () => { dragDepth = 0; document.body.classList.remove('dragging'); };
window.addEventListener('dragenter', e => { if (hasFiles(e)) { dragDepth++; document.body.classList.add('dragging'); } });
window.addEventListener('dragleave', () => { if (--dragDepth <= 0) endDrag(); });
window.addEventListener('dragover', e => { if (hasFiles(e)) e.preventDefault(); });
// never let a file drop fall through to the browser
window.addEventListener('drop', e => { if (hasFiles(e)) e.preventDefault(); endDrag(); });

document.addEventListener('keydown', e => {
  if ((e.ctrlKey || e.metaKey) && e.code === 'KeyO') { e.preventDefault(); addFiles(); }
  if (e.key === 'Escape' && openJobs.size) { openJobs.clear(); renderQueue(); }
});

let sizeTimer = 0;
window.addEventListener('resize', () => {
  clearTimeout(sizeTimer);
  sizeTimer = setTimeout(() => api.SaveWindowSize(window.innerWidth, window.innerHeight), 500);
});

window.runtime.EventsOn('state', st => {
  if ((savesInFlight || saveTimer) && state) st.settings = state.settings;
  render(st);
});
window.runtime.EventsOn('finished', r => {
  if (r.done || r.failed) toast(t('finished', r.done, r.failed), r.failed ? 'warn' : 'info');
});

api.GetState().then(st => {
  render(st);
  field = GlyphField($('#field'));
  showPage(window.__shot || 'convert');
});
