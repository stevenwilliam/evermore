const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const p = await (await b.newContext({viewport:{width:1366,height:820}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1300)
  for (const id of ['#bot-a','#bot-b']) {
    await p.locator(id).click(); await p.waitForTimeout(1900)
    const bx = await p.locator('.bot-pair').boundingBox()
    await p.screenshot({path:`${S}/open${id.slice(-1)}.png`, clip:bx})
  }
  console.log('ok')
  await b.close()
})()
