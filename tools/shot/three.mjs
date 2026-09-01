import { chromium } from 'playwright';
const b = await chromium.launch();
const p = await b.newPage();
await p.setContent('<h1>hello</h1>');
try {
  const buf = await p.screenshot({ timeout: 5000 });
  console.log('in-memory screenshot OK,', buf.length, 'bytes');
} catch (e) { console.log('in-memory FAILED:', e.message.split('\n').slice(0,3).join(' | ')); }
try {
  await p.screenshot({ path: '/home/dev/projects/evermore/docs/screenshots/_probe.png', timeout: 5000 });
  console.log('write to docs/screenshots OK');
} catch (e) { console.log('write FAILED:', e.message.split('\n').slice(0,3).join(' | ')); }
await b.close();
