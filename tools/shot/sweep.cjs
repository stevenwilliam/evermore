const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const p = await (await b.newContext({viewport:{width:1280,height:700}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.evaluate(()=>document.querySelector('#bot-spin-a').style.setProperty('--ry','-22deg'))
  await p.waitForTimeout(900)
  for (const y of [-10, 10, 30, 50]) {
    await p.evaluate(v => {
      const h = document.querySelector('#bot-a .bot-hair')
      h.style.left='50%'; h.style.top='50%'; h.style.bottom='auto'
      h.style.transformOrigin='bottom center'
      h.style.transform = `translate(-50%,-100%) translateY(${v}px) rotateX(90deg)`
    }, y)
    await p.waitForTimeout(500)
    const box = await p.locator('#bot-a .bf-lid').boundingBox()
    await p.screenshot({path:`${S}/sw${y}.png`, clip:{x:box.x-20,y:box.y-80,width:box.width+40,height:box.height+90}})
  }
  console.log('ok')
  await b.close()
})()
