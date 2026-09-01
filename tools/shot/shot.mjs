// Screenshot every public page and PROBE COMPUTED STYLES.
//
// CLAUDE.md §6: "Verify visual work by looking at it. Screenshot the rendered
// page and probe computed styles; do not conclude from reading CSS. Three
// separate defects on the previous project made text invisible at 1.00:1 and
// none was visible in the source."
//
// So this does two things a screenshot alone cannot:
//   1. It measures the real contrast of every text node against the colour
//      actually painted behind it, walking up the ancestor chain for the first
//      non-transparent background — which is how a 1.00:1 collision is found.
//   2. It checks the bar rule: anything on the mid-green must be >= 19px/700.
//
// Usage: node shot.mjs <baseURL> <outDir>

import { chromium } from 'playwright';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const BASE = process.argv[2] || 'http://127.0.0.1:8082';
const OUT = process.argv[3] || './shots';

const PAGES = [
  ['home', '/'],
  ['menu', '/menu'],
  ['packages', '/paket'],
  ['how', '/cara-kerja'],
  ['corporate', '/untuk-kantor'],
  ['coverage', '/wilayah-antar'],
  ['notfound', '/tidak-ada'],
];

const VIEWPORTS = [
  ['mobile', 360, 800],
  ['desktop', 1440, 900],
];

// --- WCAG arithmetic, mirroring scripts/contrast.py ------------------------
const probeScript = `(() => {
  function parse(c) {
    const m = c.match(/rgba?\\(([\\d.]+),\\s*([\\d.]+),\\s*([\\d.]+)(?:,\\s*([\\d.]+))?\\)/);
    if (!m) return null;
    return [ +m[1], +m[2], +m[3], m[4] === undefined ? 1 : +m[4] ];
  }
  function over(fg, bg) {
    const a = fg[3];
    return [0,1,2].map(i => Math.round(a * fg[i] + (1 - a) * bg[i])).concat([1]);
  }
  function lum(c) {
    const f = v => { v /= 255; return v <= 0.04045 ? v/12.92 : Math.pow((v+0.055)/1.055, 2.4); };
    return 0.2126*f(c[0]) + 0.7152*f(c[1]) + 0.0722*f(c[2]);
  }
  function ratio(a, b) {
    const la = lum(a), lb = lum(b);
    return (Math.max(la,lb) + 0.05) / (Math.min(la,lb) + 0.05);
  }
  // The painted background behind an element: walk up until something is not
  // transparent. Reading only the element's own background is exactly how a
  // collision hides.
  function paintedBg(el) {
    let n = el;
    while (n && n !== document.documentElement) {
      const bg = parse(getComputedStyle(n).backgroundColor);
      if (bg && bg[3] > 0) return bg[3] < 1 ? over(bg, paintedBg(n.parentElement || document.body)) : bg;
      n = n.parentElement;
    }
    const rootBg = parse(getComputedStyle(document.body).backgroundColor);
    return (rootBg && rootBg[3] > 0) ? rootBg : [255,255,255,1];
  }

  const findings = [];
  const nodes = document.querySelectorAll('body *');
  for (const el of nodes) {
    // Only elements with their own visible text.
    const own = Array.from(el.childNodes)
      .filter(n => n.nodeType === 3)
      .map(n => n.textContent.trim())
      .join(' ')
      .trim();
    if (!own) continue;
    const cs = getComputedStyle(el);
    if (cs.visibility === 'hidden' || cs.display === 'none' || +cs.opacity === 0) continue;
    const rect = el.getBoundingClientRect();
    if (rect.width < 1 || rect.height < 1) continue;

    const fgRaw = parse(cs.color);
    if (!fgRaw) continue;
    const bg = paintedBg(el);
    const fg = fgRaw[3] < 1 ? over(fgRaw, bg) : fgRaw;
    const r = ratio(fg, bg);

    const px = parseFloat(cs.fontSize);
    const weight = parseInt(cs.fontWeight, 10) || 400;
    // WCAG "large text": >= 24px, or >= 18.66px when bold (>=700).
    const large = px >= 24 || (px >= 18.66 && weight >= 700);
    const need = large ? 3.0 : 4.5;

    if (r < need) {
      findings.push({
        kind: 'contrast',
        tag: el.tagName.toLowerCase(),
        cls: (el.className || '').toString().slice(0, 40),
        text: own.slice(0, 48),
        ratio: +r.toFixed(2),
        need,
        px, weight,
        fg: 'rgb(' + fg.slice(0,3).join(',') + ')',
        bg: 'rgb(' + bg.slice(0,3).join(',') + ')',
      });
    }
  }

  // The bar rule: on #468973, every string must be >= 19px at weight >= 700.
  const barEls = document.querySelectorAll('.masthead *, .footer > .container *');
  for (const el of barEls) {
    const own = Array.from(el.childNodes).filter(n => n.nodeType === 3)
      .map(n => n.textContent.trim()).join(' ').trim();
    if (!own) continue;
    const cs = getComputedStyle(el);
    const bg = paintedBg(el);
    const isMidGreen = bg[0] === 70 && bg[1] === 137 && bg[2] === 115;
    if (!isMidGreen) continue;
    const px = parseFloat(cs.fontSize);
    const weight = parseInt(cs.fontWeight, 10) || 400;
    if (px < 19 || weight < 700) {
      findings.push({
        kind: 'bar-rule', tag: el.tagName.toLowerCase(),
        text: own.slice(0, 48), px, weight,
        note: 'on #468973 the bar rule requires >=19px and weight >=700',
      });
    }
  }

  // Horizontal overflow: the body must never scroll sideways.
  const overflow = document.documentElement.scrollWidth > window.innerWidth + 1
    ? { kind: 'overflow', scrollWidth: document.documentElement.scrollWidth, viewport: window.innerWidth }
    : null;
  if (overflow) findings.push(overflow);

  // Tap targets on mobile: interactive elements need 44px.
  if (window.innerWidth <= 480) {
    for (const el of document.querySelectorAll('a, button, input, select')) {
      const cs = getComputedStyle(el);
      if (cs.display === 'none' || cs.visibility === 'hidden') continue;
      const r = el.getBoundingClientRect();
      if (r.width < 1 || r.height < 1) continue;
      // Inline links inside a paragraph are exempt; standalone controls are not.
      const inProse = el.closest('p, li') && el.tagName === 'A';
      if (inProse) continue;
      if (r.height < 44) {
        findings.push({
          kind: 'tap-target', tag: el.tagName.toLowerCase(),
          text: (el.textContent || '').trim().slice(0, 32),
          height: +r.height.toFixed(1),
        });
      }
    }
  }
  return findings;
})()`;

const results = [];
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch();
for (const [vpName, w, h] of VIEWPORTS) {
  const ctx = await browser.newContext({
    viewport: { width: w, height: h },
    deviceScaleFactor: 1,
    colorScheme: 'light',
  });
  const page = await ctx.newPage();

  const consoleErrors = [];
  page.on('console', m => {
    // Chromium logs the main document's own 404 as a console error; the probe
    // asserts on the status separately, so that line is noise here.
    if (m.type() === 'error' && !/Failed to load resource.*404/.test(m.text())) {
      consoleErrors.push(m.text());
    }
  });
  page.on('pageerror', e => consoleErrors.push('pageerror: ' + e.message));
  const failedRequests = [];
  page.on('requestfailed', r => failedRequests.push(r.url() + ' — ' + (r.failure()?.errorText || '')));
  // A page that is SUPPOSED to 404 is not a broken sub-resource. Only count
  // responses that are not the main document.
  page.on('response', r => {
    if (r.status() >= 400 && r.request().resourceType() !== 'document') {
      failedRequests.push(r.url() + ' — HTTP ' + r.status());
    }
  });

  for (const [name, path] of PAGES) {
    consoleErrors.length = 0;
    failedRequests.length = 0;
    const resp = await page.goto(BASE + path, { waitUntil: 'networkidle', timeout: 30000 });
    // Give webfonts a moment; a screenshot taken mid-swap reports the fallback.
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(200);

    const file = join(OUT, `${name}-${vpName}.png`);
    await page.screenshot({ path: file, fullPage: true });

    const findings = await page.evaluate(probeScript);
    const fontsUsed = await page.evaluate(() => {
      const set = new Set();
      for (const el of document.querySelectorAll('h1, h2, h3, body, .masthead a')) {
        set.add(getComputedStyle(el).fontFamily.split(',')[0].replace(/["']/g, ''));
      }
      return [...set];
    });

    results.push({
      page: name, viewport: vpName, path,
      status: resp?.status(),
      file,
      findings,
      consoleErrors: [...consoleErrors],
      failedRequests: [...failedRequests],
      fontsUsed,
    });
  }
  await ctx.close();
}
await browser.close();

writeFileSync(join(OUT, 'report.json'), JSON.stringify(results, null, 2));

// --- human-readable summary -------------------------------------------------
let problems = 0;
for (const r of results) {
  const bad = r.findings.length + r.consoleErrors.length + r.failedRequests.length;
  problems += bad;
  const flag = bad ? 'FAIL' : ' ok ';
  console.log(`[${flag}] ${r.page.padEnd(10)} ${r.viewport.padEnd(8)} HTTP ${r.status}  fonts=${r.fontsUsed.join('/')}`);
  for (const f of r.findings) {
    if (f.kind === 'contrast') {
      console.log(`        contrast ${f.ratio}:1 (needs ${f.need}) ${f.px}px/${f.weight} <${f.tag} class="${f.cls}"> "${f.text}"  ${f.fg} on ${f.bg}`);
    } else if (f.kind === 'bar-rule') {
      console.log(`        bar-rule ${f.px}px/${f.weight} <${f.tag}> "${f.text}"`);
    } else if (f.kind === 'overflow') {
      console.log(`        overflow scrollWidth=${f.scrollWidth} viewport=${f.viewport}`);
    } else if (f.kind === 'tap-target') {
      console.log(`        tap-target ${f.height}px <${f.tag}> "${f.text}"`);
    }
  }
  for (const e of r.consoleErrors) console.log(`        console: ${e}`);
  for (const e of r.failedRequests) console.log(`        request: ${e}`);
}
console.log(`\n${results.length} page/viewport combinations, ${problems} problem(s).`);
process.exit(problems ? 1 : 0);
