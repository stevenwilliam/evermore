const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:6})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1500)
  const s = await p.locator('.set-ribbon-sun').first().boundingBox()
  await p.evaluate(y=>window.scrollTo(0,y-200), s.y)
  await p.waitForTimeout(400)
  const s2 = await p.locator('.set-ribbon-sun').first().boundingBox()
  const m2 = await p.locator('.set-ribbon-moon').first().boundingBox()
  await p.screenshot({path:S+'/icon-sun.png', clip:{x:s2.x-8,y:s2.y-8,width:s2.width+16,height:s2.height+16}})
  await p.screenshot({path:S+'/icon-moon.png', clip:{x:m2.x-8,y:m2.y-8,width:m2.width+16,height:m2.height+16}})
  console.log(await p.evaluate(()=>{
    const g=e=>getComputedStyle(e)
    const s=document.querySelector('.set-ribbon-sun'), m=document.querySelector('.set-ribbon-moon')
    return `sun bg ${g(s).backgroundColor} glyph ${g(s).color} | moon bg ${g(m).backgroundColor} glyph ${g(m).color}`
  }))
  await b.close()
})()
