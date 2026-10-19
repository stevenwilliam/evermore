const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S=process.argv[2]
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1500)
  const el = await p.locator('.certs').boundingBox()
  await p.evaluate(y=>window.scrollTo(0,y-40), el.y); await p.waitForTimeout(400)
  const e2 = await p.locator('.certs').boundingBox()
  await p.screenshot({path:S+'/certs.png', clip:{x:0,y:Math.max(0,e2.y-10),width:1366,height:Math.min(520,e2.height+20)}})
  console.log(await p.evaluate(()=>{
    const imgs=[...document.querySelectorAll('.cert-seal')]
    return `seals ${imgs.length}, broken ${imgs.filter(i=>!i.complete||i.naturalWidth===0).length}`
      +`, band bg ${getComputedStyle(document.querySelector('.certs')).backgroundColor}`
      +`, notes shown ${document.querySelectorAll('.cert-note').length}`
  }))
  await b.close()
})()
