/* ───────────────────────────────────────────────────────────────────────────
   SmartConfigure landing — theme, OS detection, latest-release badge.
   Loaded synchronously in <head> so the theme is applied before first paint;
   everything that touches the DOM waits for DOMContentLoaded.
   ─────────────────────────────────────────────────────────────────────────── */
(function () {
  'use strict';

  var THEME_KEY = 'smartconfigure-theme';
  var LEGACY_THEME_KEY = 'smartmatrix_theme'; // read once, what the old landing used
  var RELEASE_KEY = 'smartconfigure-release';
  var RELEASE_API = 'https://api.github.com/repos/javimcasas/smartconfigure/releases/latest';

  // ─── Theme ───────────────────────────────────────────────────────────────
  function storedTheme() {
    try {
      return localStorage.getItem(THEME_KEY) || localStorage.getItem(LEGACY_THEME_KEY) || '';
    } catch (err) { return ''; }
  }
  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme === 'dark' ? 'dark' : 'light');
  }
  function systemTheme() {
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }

  applyTheme(storedTheme() || systemTheme());

  function initThemeToggle() {
    var btn = document.getElementById('themeToggle');
    if (!btn) return;
    btn.addEventListener('click', function () {
      var next = document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
      applyTheme(next);
      try { localStorage.setItem(THEME_KEY, next); } catch (err) { /* private mode: theme just won't persist */ }
    });
    if (window.matchMedia) {
      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function () {
        if (!storedTheme()) applyTheme(systemTheme());
      });
    }
  }

  // ─── OS detection → primary download button ──────────────────────────────
  // Returns one of the data-os values used by the downloads table. Unknown
  // platforms fall back to Windows (the audience's default machine).
  function detectOs() {
    var p = '';
    try {
      p = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '';
    } catch (err) { p = ''; }
    p = p.toLowerCase();
    if (p.indexOf('mac') !== -1) return 'mac-arm64';
    if (p.indexOf('linux') !== -1 && p.indexOf('android') === -1) return 'linux';
    return 'windows';
  }

  function initDownloadCta() {
    var cta = document.getElementById('downloadCta');
    if (!cta) return;
    var os = detectOs();
    var row = document.querySelector('.downloads tr[data-os="' + os + '"]');
    if (!row) return;
    var link = row.querySelector('a[href]');
    var label = row.getAttribute('data-label') || 'Download';
    cta.setAttribute('href', link.getAttribute('href'));
    cta.querySelector('span').textContent = 'Download for ' + label;
    row.classList.add('is-current');
  }

  // ─── Latest release tag (optional decoration) ────────────────────────────
  function showRelease(data) {
    var tag = document.getElementById('versionTag');
    if (!tag || !data || !data.tag_name) return;
    var when = '';
    if (data.published_at) {
      var d = new Date(data.published_at);
      if (!isNaN(d)) when = ' · ' + d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
    }
    tag.textContent = data.tag_name + when;
    tag.hidden = false;
  }

  function initRelease() {
    if (!document.getElementById('versionTag') || typeof fetch !== 'function') return;
    var cached = null;
    try { cached = JSON.parse(sessionStorage.getItem(RELEASE_KEY) || 'null'); } catch (err) { cached = null; }
    if (cached) { showRelease(cached); return; }

    fetch(RELEASE_API, { headers: { Accept: 'application/vnd.github+json' } })
      .then(function (res) { return res.ok ? res.json() : null; })
      .then(function (json) {
        if (!json || !json.tag_name) return;
        var slim = { tag_name: json.tag_name, published_at: json.published_at };
        try { sessionStorage.setItem(RELEASE_KEY, JSON.stringify(slim)); } catch (err) { /* ignore */ }
        showRelease(slim);
      })
      .catch(function () { /* rate-limited or offline: the tag simply stays hidden */ });
  }

  document.addEventListener('DOMContentLoaded', function () {
    initThemeToggle();
    initDownloadCta();
    initRelease();
  });
})();
