const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const p = await (await b.newContext({viewport:{width:1366,height:820}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1800)
  const h = await p.locator('.hero-full').boundingBox()
  await p.screenshot({path:S+'/hero-map.png', clip:h})
  console.log(await p.evaluate(()=>{
    const m=document.querySelector('.hero-map'); const cs=getComputedStyle(m)
    const r=m.getBoundingClientRect()
    return JSON.stringify({opacity:cs.opacity, filter:cs.filter, bg:cs.backgroundColor,
      mask:(cs.maskImage||cs.webkitMaskImage||'').slice(0,46),
      w:Math.round(r.width), h:Math.round(r.height), z:cs.zIndex})
  }))
  // signature: is it writing on?
  const a = await p.evaluate(()=>getComputedStyle(document.querySelector('#week .r1')).strokeDashoffset)
  await p.waitForTimeout(2500)
  const c = await p.evaluate(()=>getComputedStyle(document.querySelector('#week .r1')).strokeDashoffset)
  console.log('sig dashoffset', a, '->', c, a===c?'STATIC':'writing')
  await b.close()
})()
