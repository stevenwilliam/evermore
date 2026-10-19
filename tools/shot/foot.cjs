const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S=process.argv[2]
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/packages', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1400)
  const f = await p.locator('.footer').boundingBox()
  await p.evaluate(y=>window.scrollTo(0,y-220), f.y); await p.waitForTimeout(400)
  await p.screenshot({path:S+'/footer.png', clip:{x:0,y:0,width:1366,height:620}})
  console.log(await p.evaluate(()=>`social links ${document.querySelectorAll('.social a').length}, svg glyphs ${document.querySelectorAll('.social a svg').length}, labels ${[...document.querySelectorAll('.social a')].map(a=>a.getAttribute('aria-label')).join('/')}`))
  await b.close()
})()
