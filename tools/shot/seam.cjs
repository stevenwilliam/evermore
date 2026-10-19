const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1600)
  for (const [sel,file] of [['#week','seam-week.png'],['.sig-pkg','seam-pkg.png']]) {
    const el = await p.locator(sel).boundingBox()
    await p.evaluate(y=>window.scrollTo(0,y), el.y-170)
    await p.waitForTimeout(500)
    await p.screenshot({path:process.argv[2]+'/'+file, clip:{x:0,y:0,width:1366,height:420}})
  }
  console.log('ok')
  await b.close()
})()
