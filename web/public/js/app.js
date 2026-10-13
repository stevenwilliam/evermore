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

      // D40: the register form carries an address and a set of allergies, so
      // a flat FormData copy is wrong twice — it would put the address fields
      // at the top level, and repeated `allergen` checkboxes would collapse to
      // whichever one came last.
      var ADDRESS_KEYS = ['label', 'recipient_name', 'recipient_phone', 'address_line',
        'district', 'city', 'province', 'postal_code', 'driver_note',
        'google_place_id', 'formatted_address'];

      var payload = {};
      var address = {};
      var hasAddress = false;
      new FormData(form).forEach(function (v, k) {
        if (k === 'allergen') return;                 // collected below
        if (ADDRESS_KEYS.indexOf(k) !== -1) { address[k] = v; hasAddress = true; return; }
        if (k === 'latitude' || k === 'longitude') { address[k] = v; return; }
        payload[k] = v;
      });

      if (hasAddress) {
        // The server wants numbers, and an empty string is not a number: send
        // null so it reports a missing pin rather than failing to parse.
        address.latitude = address.latitude === '' || address.latitude == null
          ? null : parseFloat(address.latitude);
        address.longitude = address.longitude === '' || address.longitude == null
          ? null : parseFloat(address.longitude);
        payload.address = address;
      }

      var picked = form.querySelectorAll('input[name="allergen"]:checked');
      if (picked.length) {
        payload.allergens = Array.prototype.map.call(picked, function (el) { return el.value; });
      }

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

/* D40 — the delivery-address map picker.
 *
 * The pin is the source of truth for routing; the text fields are what the
 * driver reads. latitude/longitude are NOT NULL in the schema, so the form
 * cannot be saved without a pin — and the server re-validates both, because
 * this file can be bypassed with curl.
 *
 * If Maps never loads (a referrer-restricted key on a dev host is the usual
 * reason) the manual coordinate inputs are revealed instead. That is a
 * different way to INPUT the same value, not a weaker rule: the server applies
 * exactly the same bounds either way. */
(function () {
  var form = document.querySelector('[data-address-form]');
  if (!form) return;

  var latEl = form.querySelector('[data-lat]');
  var lngEl = form.querySelector('[data-lng]');
  var manual = form.querySelector('[data-manual-latlng]');
  var manualLat = form.querySelector('[data-lat-manual]');
  var manualLng = form.querySelector('[data-lng-manual]');
  var mapEl = form.querySelector('[data-map]');
  var hint = form.querySelector('[data-map-hint]');
  var mapReady = false;

  var setPoint = function (lat, lng) {
    latEl.value = lat;
    lngEl.value = lng;
  };

  // Jakarta, as the starting view only — never submitted unless the pin moves
  // there deliberately.
  var CENTRE = { lat: -6.2088, lng: 106.8456 };

  window.evermoreInitMap = function () {
    mapReady = true;
    var map = new google.maps.Map(mapEl, {
      center: CENTRE, zoom: 12, mapTypeControl: false, streetViewControl: false
    });
    var marker = new google.maps.Marker({ map: map, position: CENTRE, draggable: true });

    var commit = function (pos) {
      setPoint(pos.lat(), pos.lng());
      if (hint) { hint.textContent = 'Pin tersimpan di ' + pos.lat().toFixed(5) + ', ' + pos.lng().toFixed(5) + '.'; }
    };
    marker.addListener('dragend', function () { commit(marker.getPosition()); });
    map.addListener('click', function (e) { marker.setPosition(e.latLng); commit(e.latLng); });

    var search = form.querySelector('[data-places-input]');
    if (search && google.maps.places) {
      var ac = new google.maps.places.Autocomplete(search, {
        componentRestrictions: { country: 'id' },
        fields: ['geometry', 'place_id', 'formatted_address', 'address_components']
      });
      ac.addListener('place_changed', function () {
        var place = ac.getPlace();
        if (!place || !place.geometry) return;
        var loc = place.geometry.location;
        map.setCenter(loc); map.setZoom(17);
        marker.setPosition(loc); commit(loc);

        var pid = form.querySelector('[data-place-id]');
        var fmt = form.querySelector('[data-formatted]');
        if (pid) pid.value = place.place_id || '';
        if (fmt) fmt.value = place.formatted_address || '';

        // Prefill the text the driver reads, but leave it editable: the pin is
        // authoritative for routing, the text is for a human.
        var byType = {};
        (place.address_components || []).forEach(function (comp) {
          comp.types.forEach(function (t) { byType[t] = comp.long_name; });
        });
        var set = function (name, v) {
          var el = form.querySelector('[name="' + name + '"]');
          if (el && !el.value && v) el.value = v;
        };
        set('address_line', place.formatted_address);
        set('district', byType.administrative_area_level_3 || byType.sublocality_level_1 || byType.locality);
        set('city', byType.administrative_area_level_2 || byType.locality);
        set('province', byType.administrative_area_level_1);
        set('postal_code', byType.postal_code);
      });
    }
  };

  // Mirror the manual inputs into the real hidden fields.
  var mirror = function () {
    if (manualLat && manualLng) setPoint(manualLat.value.trim(), manualLng.value.trim());
  };
  if (manualLat) manualLat.addEventListener('input', mirror);
  if (manualLng) manualLng.addEventListener('input', mirror);

  // Give Maps a fair chance, then fall back VISIBLY rather than leaving a dead
  // grey box and an unsubmittable form.
  window.setTimeout(function () {
    if (mapReady) return;
    if (mapEl) mapEl.hidden = true;
    if (manual) manual.hidden = false;
    if (hint) hint.textContent = 'Peta tidak tersedia — isi koordinat di bawah.';
  }, 4000);

  form.addEventListener('submit', function (ev) {
    if (!latEl.value || !lngEl.value) {
      ev.preventDefault();
      if (manual) manual.hidden = false;
      if (hint) { hint.textContent = 'Tandai lokasi di peta, atau isi koordinat, sebelum menyimpan.'; }
      (manualLat || form).focus();
    }
  });
})();

/* A destructive form asks first. */
(function () {
  document.querySelectorAll('form[data-confirm]').forEach(function (f) {
    f.addEventListener('submit', function (ev) {
      if (!window.confirm(f.getAttribute('data-confirm'))) ev.preventDefault();
    });
  });
})();

/* D41 — motion.
 *
 * Two rules this obeys:
 *   1. Nothing is hidden by CSS alone. .reveal is ADDED by this script, so a
 *      visitor with JavaScript off sees every element at full opacity instead
 *      of a blank page — the standard failure of scroll-animation libraries.
 *   2. prefers-reduced-motion is honoured by not starting at all, as well as
 *      by the duration tokens collapsing in CSS.
 */
(function () {
  var reduced = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  // ---- scroll reveal -----------------------------------------------------
  var targets = document.querySelectorAll(
    '.section-head, .card, .callout, .stats-strip, .table-wrap, .hero-inner > *, .underline-draw');

  if (!reduced && 'IntersectionObserver' in window && targets.length) {
    Array.prototype.forEach.call(targets, function (el) { el.classList.add('reveal'); });

    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        var el = entry.target;
        // Stagger siblings so a grid arrives as a sequence rather than a slab.
        var sibs = el.parentElement ? Array.prototype.indexOf.call(el.parentElement.children, el) : 0;
        el.style.setProperty('--reveal-delay', Math.min(sibs, 6) * 70 + 'ms');
        el.classList.add('in');
        io.unobserve(el);
      });
    }, { rootMargin: '0px 0px -8% 0px', threshold: 0.08 });

    Array.prototype.forEach.call(targets, function (el) { io.observe(el); });
  }

  // ---- counters ----------------------------------------------------------
  // Counts up to whatever the markup already says, so the real value is in
  // the HTML and a visitor without JS reads the final number, not a zero.
  var counters = document.querySelectorAll('.count');
  if (!reduced && 'IntersectionObserver' in window && counters.length) {
    var co = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        var el = entry.target;
        co.unobserve(el);
        var raw = (el.textContent || '').trim();
        var target = parseInt(raw.replace(/[^0-9]/g, ''), 10);
        // Anything that is not a plain integer (a time like 15:00, a price) is
        // left exactly as written.
        if (isNaN(target) || !/^[0-9]+$/.test(raw) || target === 0) return;
        var started = null, dur = 900;
        var step = function (ts) {
          if (started === null) started = ts;
          var p = Math.min((ts - started) / dur, 1);
          el.textContent = Math.round(target * (1 - Math.pow(1 - p, 3))).toString();
          if (p < 1) window.requestAnimationFrame(step);
          else el.textContent = raw;   // restore the exact original string
        };
        window.requestAnimationFrame(step);
      });
    }, { threshold: 0.5 });
    Array.prototype.forEach.call(counters, function (el) { co.observe(el); });
  }
})();

/* D45 — Bekal Bot: drag to turn it, tap to open it.
 *
 * Two gestures share one element, so the drag has to be told apart from the
 * tap or every spin would also open the box. The rule is distance: past 6px
 * of travel it is a drag, and the click that follows is swallowed.
 *
 * Opening is three stages, not two. The box fills before it scatters —
 * a container you open should look like it had something in it.
 */
(function () {
  var stage = document.getElementById('bot-stage');
  var spin = document.getElementById('bot-spin');
  if (!stage || !spin) return;

  var hint = document.getElementById('bot-hint-text');
  var reduced = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  var open = false, scatterTimer = null;
  var rx = -16, ry = -20;              // the resting three-quarter view
  var dragging = false, moved = false;
  var startX = 0, startY = 0, startRx = 0, startRy = 0;

  function apply() {
    spin.style.setProperty('--rx', rx.toFixed(2) + 'deg');
    spin.style.setProperty('--ry', ry.toFixed(2) + 'deg');
  }
  function clampX(v) { return Math.max(-62, Math.min(32, v)); }

  function setOpen(next) {
    open = next;
    window.clearTimeout(scatterTimer);
    stage.classList.toggle('is-open', open);
    stage.setAttribute('aria-pressed', open ? 'true' : 'false');
    if (open) {
      // Let the walls fall and the food read as "in the box" before it flies
      // apart. Skipped entirely under reduced motion, where the stages would
      // collapse into one frame anyway.
      if (reduced) stage.classList.add('is-scattered');
      else scatterTimer = window.setTimeout(function () {
        stage.classList.add('is-scattered');
      }, 650);
    } else {
      stage.classList.remove('is-scattered');
    }
    if (hint) hint.textContent = open ? 'untuk menutup' : 'untuk membuka';
  }

  stage.addEventListener('pointerdown', function (e) {
    dragging = true; moved = false;
    startX = e.clientX; startY = e.clientY;
    startRx = rx; startRy = ry;
    stage.classList.add('is-dragging');
    stage.setPointerCapture(e.pointerId);
  });

  stage.addEventListener('pointermove', function (e) {
    if (!dragging) return;
    var dx = e.clientX - startX, dy = e.clientY - startY;
    if (!moved && Math.abs(dx) + Math.abs(dy) > 6) moved = true;
    if (!moved) return;
    ry = startRy + dx * 0.45;
    rx = clampX(startRx - dy * 0.3);
    apply();
  });

  function endDrag(e) {
    if (!dragging) return;
    dragging = false;
    stage.classList.remove('is-dragging');
    if (e && e.pointerId !== undefined && stage.hasPointerCapture(e.pointerId)) {
      stage.releasePointerCapture(e.pointerId);
    }
  }
  stage.addEventListener('pointerup', endDrag);
  stage.addEventListener('pointercancel', endDrag);

  // A drag ends in a click event too; swallow that one so spinning the box
  // never opens it.
  stage.addEventListener('click', function (e) {
    if (moved) { moved = false; e.preventDefault(); return; }
    setOpen(!open);
  });

  // The keyboard gets the same two powers: arrows turn it, Enter and Space
  // open it (the <button> already fires click for those).
  stage.addEventListener('keydown', function (e) {
    var step = e.shiftKey ? 15 : 7;
    if (e.key === 'ArrowLeft')      { ry -= step; }
    else if (e.key === 'ArrowRight'){ ry += step; }
    else if (e.key === 'ArrowUp')   { rx = clampX(rx - step); }
    else if (e.key === 'ArrowDown') { rx = clampX(rx + step); }
    else return;
    e.preventDefault();
    apply();
  });

  apply();
})();
