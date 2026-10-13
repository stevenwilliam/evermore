const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  for (const w of [1366, 1024, 390]) {
    const p = await (await b.newContext({viewport:{width:w,height:900}, deviceScaleFactor:2})).newPage()
    await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1300)
    await p.locator('#bot-a').click(); await p.waitForTimeout(1900)
    const r = await p.evaluate(() => {
      const hero = document.querySelector('.hero-full')
      const hr = hero.getBoundingClientRect()
      const clipped = []
      document.querySelectorAll('#bot-a .bot-part').forEach((el,i)=>{
        const b = el.getBoundingClientRect()
        if (b.left < hr.left-1 || b.right > hr.right+1 || b.top < hr.top-1 || b.bottom > hr.bottom+1)
          clipped.push(i+':'+Math.round(b.left)+'..'+Math.round(b.right))
      })
      return {heroOverflow:getComputedStyle(hero).overflow, docScrollX: document.documentElement.scrollWidth > document.documentElement.clientWidth, clipped}
    })
    console.log(w+'px', JSON.stringify(r))
    if (w===1366) { const h=await p.locator('.hero-full').boundingBox(); await p.screenshot({path:S+'/herofull.png', clip:h}) }
    await p.close()
  }
  await b.close()
})()
