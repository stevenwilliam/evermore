const { chromium } = require('playwright')
const OUT = process.argv[2]
;(async () => {
  const b = await chromium.launch()
  for (const [scheme, w, h, tag] of [['light',1280,900,'desk'],['dark',1280,900,'desk'],['light',360,780,'m']]) {
    const ctx = await b.newContext({ viewport:{width:w,height:h}, colorScheme: scheme })
    const p = await ctx.newPage()
    await p.goto('http://127.0.0.1:8082/', { waitUntil:'domcontentloaded', timeout:20000 })
    await p.waitForTimeout(1600)
    await p.screenshot({ path: `${OUT}/${tag}-${scheme}-closed.png` })
    await p.click('#bot-stage')
    await p.waitForTimeout(1900)
    await p.screenshot({ path: `${OUT}/${tag}-${scheme}-open.png` })
    console.log(`  ${tag}-${scheme} both states`)
    await ctx.close()
  }
  await b.close()
})()
