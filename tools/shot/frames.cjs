const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(800)
  const shot = async (n) => {
    const s = await p.locator('#bot-a').boundingBox()
    await p.screenshot({path: process.argv[2]+'/f'+n+'.png',
      clip:{x:s.x+40, y:s.y+60, width:s.width-80, height:260}})
  }
  for (const [i,w] of [[1,0],[2,1500],[3,1500]].entries()) {}
  await shot(1); await p.waitForTimeout(1600); await shot(2); await p.waitForTimeout(1600); await shot(3)
  await p.locator('#bot-a').click(); await p.waitForTimeout(1800)
  const s = await p.locator('#bot-a').boundingBox()
  await p.screenshot({path: process.argv[2]+'/fopen.png', clip:s})
  console.log('ok')
  await b.close()
})()
