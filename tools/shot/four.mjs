import { chromium } from 'playwright';
const variants = [
  ['default', {}],
  ['no-gpu', { args: ['--disable-gpu'] }],
  ['no-sandbox+no-gpu', { args: ['--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage'] }],
  ['new-headless', { channel: 'chromium' }],
];
for (const [name, opts] of variants) {
  let b;
  try {
    b = await chromium.launch(opts);
    const p = await b.newPage();
    await p.setContent('<h1>hello</h1>');
    const buf = await p.screenshot({ timeout: 6000 });
    console.log(`${name}: OK (${buf.length} bytes)`);
    await b.close();
    break;
  } catch (e) {
    console.log(`${name}: FAILED — ${e.message.split('\n')[0]}`);
    if (b) await b.close().catch(() => {});
  }
}
