const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:3})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1500)
  const c = await p.locator('.daycard').nth(1).boundingBox()
  await p.evaluate(y=>window.scrollTo(0,y-80), c.y)
  await p.waitForTimeout(400)
  const c2 = await p.locator('.daycard').nth(1).boundingBox()
  await p.screenshot({path:process.argv[2]+'/ribbon.png', clip:{x:c2.x-6,y:c2.y-6,width:c2.width+12,height:c2.height+12}})
  console.log(await p.evaluate(()=>{
    const s=document.querySelector('.set-ribbon-sun'), m=document.querySelector('.set-ribbon-moon')
    const g=e=>getComputedStyle(e)
    return `sun bg ${g(s).backgroundColor} glyph ${g(s).color} | moon bg ${g(m).backgroundColor} glyph ${g(m).color}`
      +` | sun size ${Math.round(s.getBoundingClientRect().width)}px`
      +` | hidden labels ${document.querySelectorAll('.daycard-set .visually-hidden').length}`
      +` | label text still visible? ${[...document.querySelectorAll('.daycard-set')].some(e=>/LUNCH|DINNER/i.test(e.innerText))}`
  }))
  await b.close()
})()
