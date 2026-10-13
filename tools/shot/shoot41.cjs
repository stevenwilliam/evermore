const { chromium } = require('playwright')
const OUT = process.argv[2]
;(async () => {
  const b = await chromium.launch()
  const routes = [['home','/'],['menu','/menu'],['paket','/paket'],['how','/cara-kerja']]
  for (const scheme of ['light','dark']) {
    const ctx = await b.newContext({ viewport:{width:1280,height:900}, colorScheme: scheme })
    const p = await ctx.newPage()
    for (const [name, path] of routes) {
      const r = await p.goto('http://127.0.0.1:8082'+path, {waitUntil:'domcontentloaded', timeout:20000})
      await p.waitForTimeout(1800)   // let reveals finish
      await p.screenshot({ path: `${OUT}/${name}-${scheme}.png`, timeout: 20000 })
      console.log(`  ${name}-${scheme} (${r.status()})`)
    }
    await ctx.close()
  }
  const m = await b.newContext({ viewport:{width:360,height:780} })
  const mp = await m.newPage()
  await mp.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded'})
  await mp.waitForTimeout(1800)
  await mp.screenshot({ path: `${OUT}/home-360.png`, timeout: 20000 })
  console.log('  home-360')
  await b.close()
})()
