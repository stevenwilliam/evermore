import { chromium } from 'playwright';
const b = await chromium.launch();
const p = await b.newPage();
// 1. A trivial page — does screenshotting work at all right now?
await p.setContent('<h1>hello</h1>');
try { await p.screenshot({ path: '/tmp/t1.png', timeout: 5000 }); console.log('trivial page: OK'); }
catch (e) { console.log('trivial page: FAILED'); }
// 2. The live page with CSS disabled — is it the stylesheet?
await p.goto('http://192.168.88.101:8091/', { waitUntil: 'domcontentloaded' });
await p.addStyleTag({ content: '*{animation:none!important;transition:none!important}' });
try { await p.screenshot({ path: '/tmp/t2.png', timeout: 5000 }); console.log('live page, motion killed: OK'); }
catch (e) { console.log('live page, motion killed: FAILED'); }
// 3. What is actually animating?
const anims = await p.evaluate(() => document.getAnimations().map(a => ({
  name: a.animationName || a.transitionProperty || a.constructor.name,
  state: a.playState,
  target: a.effect?.target?.tagName + '.' + (a.effect?.target?.className || ''),
})));
console.log('running animations:', JSON.stringify(anims.slice(0, 8)));
await b.close();
