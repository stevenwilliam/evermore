import { chromium } from 'playwright';
const b = await chromium.launch();
const ctx = await b.newContext({ viewport: { width: 360, height: 800 } });
const p = await ctx.newPage();
await p.goto('http://192.168.88.101:8091/', { waitUntil: 'networkidle' });
console.log('loaded, scrollHeight =', await p.evaluate(() => document.documentElement.scrollHeight));
try {
  await p.screenshot({ path: '/tmp/one-full.png', fullPage: true, timeout: 8000 });
  console.log('fullPage OK');
} catch (e) { console.log('fullPage FAILED:', e.message.split('\n')[0]); }
try {
  await p.screenshot({ path: '/tmp/one-vp.png', timeout: 8000 });
  console.log('viewport OK');
} catch (e) { console.log('viewport FAILED:', e.message.split('\n')[0]); }
await b.close();
