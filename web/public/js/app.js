/* Evermore — the small amount of JavaScript the public site needs.
 *
 * Everything here is an enhancement. The nav is a real <nav>, the search is a
 * real GET form, and every page works with this file blocked. */
(function () {
  'use strict';

  // --- mobile navigation -------------------------------------------------
  var toggle = document.querySelector('[data-nav-toggle]');
  var nav = document.getElementById('mobile-nav');
  if (toggle && nav) {
    toggle.addEventListener('click', function () {
      var open = toggle.getAttribute('aria-expanded') === 'true';
      toggle.setAttribute('aria-expanded', String(!open));
      // Toggle the property, not a class: [hidden] can be overridden by a
      // stylesheet, and el.hidden cannot be argued with.
      nav.hidden = open;
    });
  }

  // --- auth forms --------------------------------------------------------
  // The login and register forms post JSON to the API and then navigate. The
  // refresh token comes back as an HttpOnly cookie, so nothing sensitive is
  // ever reachable from JavaScript; the short-lived access token is kept in
  // memory only, and the page reload re-reads it from a refresh.
  document.querySelectorAll('[data-auth-form]').forEach(function (form) {
    var errBox = form.querySelector('[data-auth-error]');
    form.addEventListener('submit', function (ev) {
      ev.preventDefault();
      if (errBox) { errBox.hidden = true; errBox.textContent = ''; }

      var payload = {};
      new FormData(form).forEach(function (v, k) { payload[k] = v; });

      fetch(form.getAttribute('data-endpoint'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify(payload)
      }).then(function (res) {
        return res.json().catch(function () { return {}; }).then(function (body) {
          return { ok: res.ok, body: body };
        });
      }).then(function (r) {
        if (!r.ok) {
          var msg = (r.body && r.body.error && r.body.error.message)
            || 'Tidak bisa diproses. Coba lagi.';
          if (errBox) { errBox.textContent = msg; errBox.hidden = false; }
          return;
        }
        window.location.assign(form.getAttribute('data-next') || '/');
      }).catch(function () {
        if (errBox) {
          errBox.textContent = 'Jaringan bermasalah. Coba lagi.';
          errBox.hidden = false;
        }
      });
    });
  });

  // --- debounced live search --------------------------------------------
  // CLAUDE.md §7 wants the search box debounced. The form still submits
  // normally without JS; this filters the already-rendered cards so typing
  // does not cost a round trip.
  var form = document.querySelector('[data-live-search]');
  if (!form) return;
  var input = form.querySelector('input[type="search"]');
  var results = document.querySelector('[data-results]');
  if (!input || !results) return;

  var counter = results.querySelector('[data-result-count]');
  var cards = Array.prototype.slice.call(results.querySelectorAll('[data-search-text]'));
  var timer = null;

  function normalise(s) {
    return (s || '').toLowerCase().normalize('NFC').trim();
  }

  function apply() {
    var q = normalise(input.value);
    var shown = 0;
    cards.forEach(function (card) {
      var hay = normalise(card.getAttribute('data-search-text') + ' ' + card.textContent);
      var match = q === '' || hay.indexOf(q) !== -1;
      card.hidden = !match;
      if (match) shown++;
    });
    if (counter) {
      counter.textContent = shown === cards.length
        ? shown + ' menu ditemukan'
        : shown + ' dari ' + cards.length + ' menu cocok';
    }
    // Announce the change for screen readers rather than silently reflowing.
    if (counter) counter.setAttribute('aria-live', 'polite');
  }

  input.addEventListener('input', function () {
    window.clearTimeout(timer);
    timer = window.setTimeout(apply, 200);
  });
})();

/* D34 — the Thenie chrome: the floating bottom nav and the sticky mark.
 * Both are progressive: with JS off the links list is still in the DOM and
 * reachable, and the mark simply never appears. */
(function () {
  var toggle = document.querySelector('[data-float-nav-toggle]');
  var links = document.getElementById('float-nav-links');
  if (toggle && links) {
    // The list ships hidden so a JS-less visit does not get a panel floating
    // over the page; revealing it here is what makes the toggle meaningful.
    links.hidden = false;
    links.classList.remove('open');

    var setOpen = function (open) {
      toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
      links.classList.toggle('open', open);
    };
    toggle.addEventListener('click', function () {
      setOpen(toggle.getAttribute('aria-expanded') !== 'true');
    });
    // Escape closes it and returns focus, or the keyboard user is stranded
    // inside a panel with no way back to the control that opened it.
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && toggle.getAttribute('aria-expanded') === 'true') {
        setOpen(false);
        toggle.focus();
      }
    });
    document.addEventListener('click', function (e) {
      if (!links.contains(e.target) && !toggle.contains(e.target)) setOpen(false);
    });
  }

  var mark = document.querySelector('[data-sticky-logo]');
  if (mark) {
    // Appears once the hero has scrolled past. Falls back to a fixed offset
    // on pages with no hero, so it still behaves on /menu and /paket.
    var hero = document.querySelector('.hero-full');
    var trigger = function () { return hero ? hero.offsetHeight - 120 : 200; };
    var sync = function () {
      var on = window.scrollY > trigger();
      mark.classList.toggle('show', on);
      mark.setAttribute('aria-hidden', on ? 'false' : 'true');
    };
    window.addEventListener('scroll', sync, { passive: true });
    window.addEventListener('resize', sync);
    sync();
  }
})();
