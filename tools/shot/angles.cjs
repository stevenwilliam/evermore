const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const p = await (await b.newContext({viewport:{width:1280,height:700}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1200)
  for (const ry of [-22, 20, 50, 98]) {
    await p.evaluate(v => {
      const s = document.querySelector('#bot-spin-a')
      s.style.setProperty('--ry', v + 'deg')
    }, ry)
    await p.waitForTimeout(700)
    const box = await p.locator('#bot-a').boundingBox()
    await p.screenshot({path: `${S}/ry${ry}.png`, clip:{x:box.x+30,y:box.y+50,width:box.width-60,height:250}})
  }
  console.log('ok')
  await b.close()
})()
