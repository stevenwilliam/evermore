// Prove the gradient blind-spot guard fires. It is tested against a synthetic
// page rather than the app, because the app not having the defect proves
// nothing about whether the guard can see it.
import { chromium } from 'playwright';
import { readFileSync } from 'node:fs';

// Reuse the REAL probe source, not a copy — a copy drifts from what ships.
const src = readFileSync('./shot.mjs', 'utf8');
const m = src.match(/const probeScript = `([\s\S]*?)`;\n/);
if (!m) { console.log('FAIL: could not extract probeScript from shot.mjs'); process.exit(1); }
const probeScript = m[1].replace(/\\\\/g, '\\').replace(/\\`/g, '`').replace(/\\\$/g, '$');

const b = await chromium.launch();
const p = await b.newPage();

const cases = [
  ['gradient WITHOUT background-color (should FIRE)',
   `<div style="background:#fff"><div class="probe" style="background-image:linear-gradient(#468973,#1C3D34);color:#FFFAE0;font:700 19px sans-serif;padding:40px">foto meal</div></div>`,
   true],
  ['gradient WITH background-color (should be SILENT)',
   `<div style="background:#fff"><div class="probe" style="background-color:#468973;background-image:linear-gradient(#468973,#1C3D34);color:#FFFAE0;font:700 19px sans-serif;padding:40px">foto meal</div></div>`,
   false],
];

let ok = true;
for (const [label, html, shouldFire] of cases) {
  await p.setContent(html);
  const findings = await p.evaluate(probeScript);
  const fired = findings.some(f => f.kind === 'gradient-no-bgcolor');
  const pass = fired === shouldFire;
  if (!pass) ok = false;
  console.log(`  ${pass ? 'PASS' : 'FAIL'}  ${label} -> guard ${fired ? 'fired' : 'silent'}`);
  if (fired) console.log(`         ${findings.filter(f=>f.kind==='gradient-no-bgcolor').map(f=>f.tag).join(', ')}`);
}
await b.close();
console.log(ok ? '\nThe guard works.' : '\nTHE GUARD IS BROKEN.');
process.exit(ok ? 0 : 1);
