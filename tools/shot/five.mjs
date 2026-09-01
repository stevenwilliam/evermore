import { chromium } from 'playwright';
// The older cached revision, and the full chromium rather than headless_shell.
const cands = [
  ['1148 headless_shell', process.env.HOME + '/.cache/ms-playwright/chromium_headless_shell-1148/chrome-linux/headless_shell'],
  ['1148 chrome',         process.env.HOME + '/.cache/ms-playwright/chromium-1148/chrome-linux/chrome'],
  ['1234 chrome',         process.env.HOME + '/.cache/ms-playwright/chromium-1234/chrome-linux/chrome'],
];
for (const [name, exe] of cands) {
  let b;
  try {
    b = await chromium.launch({ executablePath: exe });
    const p = await b.newPage();
    await p.setContent('<h1>hello</h1>');
    const buf = await p.screenshot({ timeout: 6000 });
    console.log(`${name}: OK (${buf.length} bytes)  exe=${exe}`);
    await b.close();
    break;
  } catch (e) {
    console.log(`${name}: ${e.message.split('\n')[0]}`);
    if (b) await b.close().catch(() => {});
  }
}
