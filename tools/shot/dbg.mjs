import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 1440, height: 900 } });
const p = await ctx.newPage();
await p.goto('http://192.168.88.101:8091/tidak-ada', { waitUntil: 'networkidle' });
const info = await p.evaluate(() => {
  const box = el => { const r = el.getBoundingClientRect(); const cs = getComputedStyle(el);
    return { tag: el.tagName.toLowerCase() + (el.className ? '.' + String(el.className).split(' ')[0] : ''),
             top: Math.round(r.top), bottom: Math.round(r.bottom), h: Math.round(r.height),
             display: cs.display, position: cs.position, flex: cs.flex, margin: cs.margin }; };
  const bodyCS = getComputedStyle(document.body);
  return {
    viewportH: window.innerHeight,
    bodyH: Math.round(document.body.getBoundingClientRect().height),
    bodyDisplay: bodyCS.display, bodyMinH: bodyCS.minHeight,
    htmlH: Math.round(document.documentElement.getBoundingClientRect().height),
    children: [...document.body.children].map(box),
  };
});
console.log(JSON.stringify(info, null, 1));
await b.close();
