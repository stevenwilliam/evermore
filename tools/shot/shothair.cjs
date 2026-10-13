const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}, deviceScaleFactor:3})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1600)
  const h = await p.locator('#bot-a .bot-hair').boundingBox()
  await p.screenshot({ path: process.argv[2]+'/hair.png',
    clip:{x:h.x-60, y:h.y-14, width:h.width+120, height:h.height+70} })
  console.log('hair box', JSON.stringify(h))
  await b.close()
})()
