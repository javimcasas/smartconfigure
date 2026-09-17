/* ───────────────────────────────────────────────────────────────────────────
   SmartConfigure Log Viewer — parse + render, all in the browser.
   parseLog() mirrors the exact structure written by internal/sshrunner
   (writeHeader / writeSection / writeFooter, and DryRun): change both
   together. Nothing here is stored anywhere.
   ─────────────────────────────────────────────────────────────────────────── */
(function () {
  'use strict';

  var SEP_RE = /^─+\s*$/;
  var DEVICE_ERROR_RE = /% Unrecognized command|% Wrong parameter|Error: |% Invalid|Unrecognized command found/i;
  var OPEN_FAILED_MAX = 5; // failed devices are rendered open when there are this many or fewer

  // ─── Parsing ─────────────────────────────────────────────────────────────
  function parseLog(text) {
    var lines = text.replace(/\r\n/g, '\n').split('\n');
    var i = 0;

    var titleLine = (lines[i++] || '').trim();
    if (!/^SmartConfigure/.test(titleLine)) return null;
    var isDryRun = /DRY RUN/i.test(titleLine);

    var kv = {};
    while (i < lines.length && !SEP_RE.test(lines[i])) {
      var m = lines[i].match(/^([A-Za-z][A-Za-z ]*?):\s+(.*)$/);
      if (m) kv[m[1].trim()] = m[2].trim();
      i++;
    }
    if (!kv['Device']) return null;

    var header = {
      title: titleLine,
      isDryRun: isDryRun,
      device: kv['Device'],
      user: kv['User'] || '',
      port: kv['Port'] || '',
      templateSize: kv['Template size'] || '',
      started: kv['Started'] || ''
    };

    // The header ends with exactly one separator, then a blank line. Consume
    // just those — the next separator already belongs to the first section.
    if (i < lines.length && SEP_RE.test(lines[i])) i++;
    while (i < lines.length && lines[i].trim() === '') i++;

    var sections = [];
    var footer = null;

    while (i < lines.length) {
      if (!SEP_RE.test(lines[i])) { i++; continue; }
      i++;
      if (i >= lines.length) break;

      if (/^RESULT:/.test(lines[i])) {
        var resultLine = lines[i++];
        var fkv = {};
        while (i < lines.length && !SEP_RE.test(lines[i])) {
          var fm = lines[i].match(/^([A-Za-z]+):\s+(.*)$/);
          if (fm) fkv[fm[1].trim()] = fm[2].trim();
          i++;
        }
        footer = {
          success: /RESULT:\s*(SUCCESS|DRY-RUN OK)/i.test(resultLine),
          resultText: resultLine.replace(/^RESULT:\s*/, '').trim(),
          finished: fkv['Finished'] || '',
          duration: fkv['Duration'] || ''
        };
        break;
      }

      var tLine = lines[i++] || '';
      var tm = tLine.match(/^(\d{2}:\d{2}:\d{2}\.\d{3})\s+(.*)$/);
      var timestamp = tm ? tm[1] : '';
      var title = tm ? tm[2] : tLine;

      if (i < lines.length && SEP_RE.test(lines[i])) i++;

      var bodyLines = [];
      while (i < lines.length && !SEP_RE.test(lines[i])) {
        bodyLines.push(lines[i]);
        i++;
      }
      while (bodyLines.length && bodyLines[bodyLines.length - 1].trim() === '') bodyLines.pop();

      var body = bodyLines.join('\n');
      var isError = /^STOPPED$/i.test(title) || /FAILED/i.test(title) || DEVICE_ERROR_RE.test(body);
      sections.push({ timestamp: timestamp, title: title, body: body, isError: isError });
    }

    return { header: header, sections: sections, footer: footer };
  }

  // report.csv is written by encoding/csv with 4 columns; Device/Status/
  // Duration never contain commas, Error may — so the tail is re-joined.
  function parseCsv(text) {
    var lines = text.replace(/\r\n/g, '\n').split('\n').filter(function (l) { return l.trim() !== ''; });
    if (lines.length < 1 || !/^Device,Status,Duration,Error/i.test(lines[0])) return null;
    return lines.slice(1).map(function (line) {
      var parts = line.split(',');
      return {
        device: parts[0] || '',
        status: parts[1] || '',
        duration: parts[2] || '',
        error: parts.slice(3).join(',').replace(/^"|"$/g, '')
      };
    });
  }

  function parseDurationMs(str) {
    if (!str) return 0;
    var ms = 0, matched = false;
    var re = /([\d.]+)(h|ms|m|s|µs|ns)/g, m;
    while ((m = re.exec(str))) {
      matched = true;
      var n = parseFloat(m[1]);
      if (m[2] === 'h') ms += n * 3600000;
      else if (m[2] === 'm') ms += n * 60000;
      else if (m[2] === 's') ms += n * 1000;
      else if (m[2] === 'ms') ms += n;
      // µs / ns are below display resolution
    }
    return matched ? ms : (parseFloat(str) || 0);
  }

  function formatMs(ms) {
    if (ms < 1000) return Math.round(ms) + 'ms';
    var s = ms / 1000;
    if (s < 60) return s.toFixed(1) + 's';
    var mins = Math.floor(s / 60);
    return mins + 'm ' + Math.round(s % 60) + 's';
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  function ipKey(ip) {
    var parts = ip.split('.');
    if (parts.length !== 4) return ip;
    return parts.map(function (p) { return ('000' + p).slice(-3); }).join('.');
  }

  // ─── State ───────────────────────────────────────────────────────────────
  var state = { logs: [], csvRows: null, filter: 'all' };

  var $ = function (id) { return document.getElementById(id); };
  var els = {
    dropzone: $('dropzone'), fileInput: $('fileInput'), browseBtn: $('browseBtn'), status: $('dropStatus'),
    results: $('results'), runStrip: $('runStrip'), clearBtn: $('clearBtn'),
    csvSection: $('csvSection'), csvBody: $('csvTable').querySelector('tbody'),
    devices: $('devices'), devicesEmpty: $('devicesEmpty'), toggleAllBtn: $('toggleAllBtn'),
    cntAll: $('cntAll'), cntOk: $('cntOk'), cntFail: $('cntFail')
  };

  function statusOf(log) {
    var f = log.footer, h = log.header;
    if (!f) return { key: 'fail', badge: 'neutral', text: 'INCOMPLETE', ok: false };
    if (h.isDryRun) {
      return f.success
        ? { key: 'ok', badge: 'neutral', text: 'DRY-RUN OK', ok: true }
        : { key: 'fail', badge: 'fail', text: 'DRY-RUN FAILED', ok: false };
    }
    return f.success
      ? { key: 'ok', badge: 'ok', text: 'OK', ok: true }
      : { key: 'fail', badge: 'fail', text: 'FAILED', ok: false };
  }

  // ─── Rendering ───────────────────────────────────────────────────────────
  function render() {
    var hasData = state.logs.length > 0 || (state.csvRows && state.csvRows.length > 0);
    els.results.hidden = !hasData;
    if (!hasData) return;

    var logs = state.logs.slice();
    if (state.csvRows) logs.sort(function (a, b) { return ipKey(a.header.device) < ipKey(b.header.device) ? -1 : 1; });

    var ok = 0, fail = 0, totalMs = 0;
    logs.forEach(function (l) {
      var s = statusOf(l);
      if (s.ok) ok++; else fail++;
      if (l.footer) totalMs += parseDurationMs(l.footer.duration);
    });

    els.runStrip.innerHTML =
      '<div><div class="num">' + logs.length + '</div><div class="lbl">devices</div></div>' +
      '<div><div class="num ok">' + ok + '</div><div class="lbl">OK</div></div>' +
      '<div><div class="num fail">' + fail + '</div><div class="lbl">failed</div></div>' +
      '<div><div class="num">' + formatMs(totalMs) + '</div><div class="lbl">total</div></div>';
    els.cntAll.textContent = logs.length;
    els.cntOk.textContent = ok;
    els.cntFail.textContent = fail;

    renderCsv();

    els.devices.innerHTML = '';
    var openFailed = fail > 0 && fail <= OPEN_FAILED_MAX;
    logs.forEach(function (log) {
      var s = statusOf(log);
      els.devices.appendChild(buildDevice(log, s, openFailed && !s.ok));
    });
    applyFilter();
  }

  function renderCsv() {
    var rows = state.csvRows;
    els.csvSection.hidden = !(rows && rows.length);
    if (!rows || !rows.length) return;
    els.csvBody.innerHTML = rows.map(function (r) {
      var isOk = /^OK$/i.test(r.status);
      return '<tr><td class="mono">' + escapeHtml(r.device) + '</td>' +
        '<td><span class="badge ' + (isOk ? 'ok' : 'fail') + '">' + escapeHtml(r.status) + '</span></td>' +
        '<td class="mono">' + escapeHtml(r.duration) + '</td>' +
        '<td class="wrap">' + (r.error ? escapeHtml(r.error) : '<span class="muted">—</span>') + '</td></tr>';
    }).join('');
  }

  function buildDevice(log, s, open) {
    var h = log.header, f = log.footer;
    var el = document.createElement('details');
    el.className = 'device';
    el.dataset.status = s.key;
    if (open) el.open = true;

    var meta = [];
    if (h.user) meta.push(escapeHtml(h.user));
    if (h.port) meta.push('port ' + escapeHtml(h.port));
    if (h.templateSize) meta.push(escapeHtml(h.templateSize));
    if (f && f.duration) meta.push(escapeHtml(f.duration));

    var sectionsHtml = log.sections.map(function (sec) {
      return '<div class="tsec' + (sec.isError ? ' err' : '') + '">' +
        '<div class="tsec-head">' +
          (sec.timestamp ? '<span class="tsec-time">' + escapeHtml(sec.timestamp) + '</span>' : '') +
          '<span class="tsec-title">' + escapeHtml(sec.title) + '</span>' +
        '</div>' +
        '<pre>' + escapeHtml(sec.body || '(no output)') + '</pre>' +
      '</div>';
    }).join('');

    var footHtml = f
      ? '<span>Started ' + escapeHtml(h.started) + '</span><span>Finished ' + escapeHtml(f.finished) + '</span>' +
        (f.resultText ? '<span>' + escapeHtml(f.resultText) + '</span>' : '')
      : '<span>Started ' + escapeHtml(h.started) + '</span><span>Log ended before the RESULT block.</span>';

    el.innerHTML =
      '<summary>' +
        '<span class="device-dot ' + (s.ok ? 'ok' : 'fail') + '" aria-hidden="true"></span>' +
        '<span class="device-main">' +
          '<span class="device-ip">' + escapeHtml(h.device) + '</span>' +
          '<span class="device-meta">' + meta.map(function (m) { return '<span>' + m + '</span>'; }).join('') + '</span>' +
        '</span>' +
        '<span class="badge ' + s.badge + '">' + s.text + '</span>' +
        '<svg class="device-chev" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m6 9 6 6 6-6"/></svg>' +
      '</summary>' +
      '<div class="device-body">' + sectionsHtml + '<div class="device-foot">' + footHtml + '</div></div>';

    // When a failed device is opened by hand, bring its first error into view.
    el.addEventListener('toggle', function () {
      if (!el.open || s.ok) return;
      var firstErr = el.querySelector('.tsec.err');
      if (firstErr && !el.dataset.scrolled) {
        el.dataset.scrolled = '1';
        firstErr.scrollIntoView({ block: 'nearest' });
      }
    });
    return el;
  }

  function applyFilter() {
    var shown = 0;
    Array.prototype.forEach.call(els.devices.children, function (el) {
      var visible = state.filter === 'all' || el.dataset.status === state.filter;
      el.hidden = !visible;
      if (visible) shown++;
    });
    els.devicesEmpty.hidden = shown > 0;
    updateToggleAll();
  }

  function updateToggleAll() {
    var visible = Array.prototype.filter.call(els.devices.children, function (el) { return !el.hidden; });
    var allOpen = visible.length > 0 && visible.every(function (el) { return el.open; });
    els.toggleAllBtn.textContent = allOpen ? 'Collapse all' : 'Expand all';
    els.toggleAllBtn.dataset.mode = allOpen ? 'collapse' : 'expand';
  }

  // ─── File handling ───────────────────────────────────────────────────────
  function setStatus(text, isAlert) {
    els.status.textContent = text;
    if (isAlert) els.status.setAttribute('role', 'alert'); else els.status.removeAttribute('role');
  }

  function readFile(file) {
    return new Promise(function (resolve) {
      var reader = new FileReader();
      reader.onload = function (e) { resolve({ name: file.name, text: String(e.target.result || '') }); };
      reader.onerror = function () { resolve({ name: file.name, text: null }); };
      reader.readAsText(file);
    });
  }

  function handleFiles(fileList) {
    var files = Array.prototype.slice.call(fileList || []);
    if (!files.length) return;

    Promise.all(files.map(readFile)).then(function (loaded) {
      var skipped = [], added = 0;
      loaded.forEach(function (f) {
        if (f.text === null) { skipped.push(f.name); return; }
        if (/\.csv$/i.test(f.name)) {
          var rows = parseCsv(f.text);
          if (rows) { state.csvRows = rows; added++; } else skipped.push(f.name);
          return;
        }
        var log = parseLog(f.text);
        if (log) { state.logs.push(log); added++; } else skipped.push(f.name);
      });

      render();
      if (skipped.length) {
        setStatus(skipped.length + (skipped.length === 1 ? ' file' : ' files') + ' skipped: ' + skipped.join(', ') +
          ' — not SmartConfigure logs.', true);
      } else {
        setStatus(added + (added === 1 ? ' file' : ' files') + ' loaded.', false);
      }
    });
  }

  // ─── Wiring ──────────────────────────────────────────────────────────────
  els.browseBtn.addEventListener('click', function () { els.fileInput.click(); });
  els.fileInput.addEventListener('change', function (e) { handleFiles(e.target.files); e.target.value = ''; });

  // The whole window is a drop target; the zone just lights up.
  var dragDepth = 0;
  window.addEventListener('dragenter', function (e) { e.preventDefault(); dragDepth++; els.dropzone.classList.add('is-over'); });
  window.addEventListener('dragover', function (e) { e.preventDefault(); });
  window.addEventListener('dragleave', function () { dragDepth = Math.max(0, dragDepth - 1); if (!dragDepth) els.dropzone.classList.remove('is-over'); });
  window.addEventListener('drop', function (e) {
    e.preventDefault();
    dragDepth = 0;
    els.dropzone.classList.remove('is-over');
    handleFiles(e.dataTransfer && e.dataTransfer.files);
  });

  els.clearBtn.addEventListener('click', function () {
    state = { logs: [], csvRows: null, filter: 'all' };
    document.querySelector('input[name="filter"][value="all"]').checked = true;
    els.devices.innerHTML = '';
    render();
    setStatus('No files loaded yet.', false);
    els.browseBtn.focus();
  });

  Array.prototype.forEach.call(document.querySelectorAll('input[name="filter"]'), function (input) {
    input.addEventListener('change', function () { state.filter = input.value; applyFilter(); });
  });

  els.toggleAllBtn.addEventListener('click', function () {
    var open = els.toggleAllBtn.dataset.mode !== 'collapse';
    Array.prototype.forEach.call(els.devices.children, function (el) { if (!el.hidden) el.open = open; });
    updateToggleAll();
  });
  els.devices.addEventListener('toggle', updateToggleAll, true);
})();
