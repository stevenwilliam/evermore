import { chromium } from 'playwright';
const BASE = process.argv[2] || 'http://192.168.88.101:8091';
const b = await chromium.launch();

for (const [vp, w, h] of [['mobile', 360, 800], ['desktop', 1440, 900]]) {
  const ctx = await b.newContext({ viewport: { width: w, height: h } });
  const p = await ctx.newPage();

  // A deliberately SHORT page, where a non-sticky footer floats up.
  await p.goto(BASE + '/tidak-ada', { waitUntil: 'networkidle' });
  const short = await p.evaluate(() => {
    // Measure the FOOTER element, not the fine print inside it: the footer's
    // own padding sits below its last child, so measuring the child reports a
    // gap that is not there.
    const f = document.querySelector('.footer').getBoundingClientRect();
    return {
      contentH: document.documentElement.scrollHeight,
      viewportH: window.innerHeight,
      footerBottom: Math.round(f.bottom),
      docBottom: Math.round(document.documentElement.getBoundingClientRect().bottom),
      scrolls: document.documentElement.scrollHeight > window.innerHeight + 1,
    };
  });
  // Two different correct outcomes. On a page SHORTER than the viewport the
  // footer must sit on the bottom edge — that is what sticky means. On a page
  // taller than the viewport it correctly sits after the content, and the
  // thing to assert is that nothing renders below it.
  const target = short.scrolls ? short.docBottom : short.viewportH;
  const gap = target - short.footerBottom;
  const what = short.scrolls ? 'document bottom (page scrolls)' : 'viewport bottom (page fits)';
  console.log(`[${vp}] 404 page: content ${short.contentH}, viewport ${short.viewportH}, footer bottom ${short.footerBottom}, gap ${gap}px vs ${what}`);
  console.log(`        ${Math.abs(gap) <= 1 ? 'PASS' : 'FAIL'} — footer is flush with the ${what}`);

  // The float: measured, not eyeballed.
  const wa = await p.evaluate(() => {
    const el = document.querySelector('.wa-float');
    if (!el) return null;
    const cs = getComputedStyle(el);
    const r = el.getBoundingClientRect();
    const svg = el.querySelector('svg');
    return {
      position: cs.position,
      fill: cs.backgroundColor,
      glyph: cs.color,
      shadow: cs.boxShadow,
      w: Math.round(r.width), h: Math.round(r.height),
      fromRight: Math.round(window.innerWidth - r.right),
      fromBottom: Math.round(window.innerHeight - r.bottom),
      inViewport: r.bottom <= window.innerHeight && r.right <= window.innerWidth,
      label: el.textContent.trim(),
      href: el.getAttribute('href'),
      target: el.getAttribute('target'),
      rel: el.getAttribute('rel'),
      svgHidden: svg && svg.getAttribute('aria-hidden'),
    };
  });
  if (!wa) { console.log(`[${vp}] NO WhatsApp float found`); continue; }
  console.log(`[${vp}] float: ${wa.position} ${wa.w}x${wa.h}px, ${wa.fromRight}px from right, ${wa.fromBottom}px from bottom`);
  console.log(`        fill ${wa.fill}, glyph ${wa.glyph}`);
  console.log(`        ${wa.w >= 44 && wa.h >= 44 ? 'PASS' : 'FAIL'} — touch target >= 44px`);
  console.log(`        ${wa.position === 'fixed' && wa.inViewport ? 'PASS' : 'FAIL'} — fixed and inside the viewport`);
  console.log(`        ${wa.fill === 'rgb(18, 140, 126)' ? 'PASS' : 'FAIL'} — fill is WhatsApp #128C7E, not recoloured`);
  console.log(`        ${/rgb\(255, 250, 224\)/.test(wa.shadow) ? 'PASS' : 'FAIL'} — beige ring present (the affordance)`);
  console.log(`        ${wa.label ? 'PASS' : 'FAIL'} — accessible name: "${wa.label}"`);
  console.log(`        ${wa.rel && wa.rel.includes('noopener') ? 'PASS' : 'FAIL'} — rel=noopener on target=_blank`);
  console.log(`        ${wa.svgHidden === 'true' ? 'PASS' : 'FAIL'} — decorative svg hidden from AT`);

  // Does it cover a footer link on a long page?
  await p.goto(BASE + '/', { waitUntil: 'networkidle' });
  await p.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
  await p.waitForTimeout(300);
  const overlap = await p.evaluate(() => {
    const f = document.querySelector('.wa-float').getBoundingClientRect();
    const hits = [];
    for (const a of document.querySelectorAll('.footer a, .footer-fineprint a')) {
      const r = a.getBoundingClientRect();
      if (r.width && r.height && !(r.right < f.left || r.left > f.right || r.bottom < f.top || r.top > f.bottom)) {
        hits.push(a.textContent.trim().slice(0, 30));
      }
    }
    return hits;
  });
  console.log(`        ${overlap.length === 0 ? 'PASS' : 'FAIL'} — covers no footer link${overlap.length ? ': ' + overlap.join(', ') : ''}`);
  await ctx.close();
}
await b.close();
