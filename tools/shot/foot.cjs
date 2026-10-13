const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  for (const w of [1366, 1024, 390]) {
    const p = await (await b.newContext({viewport:{width:w,height:900}, deviceScaleFactor:2})).newPage()
    await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1600)
    const el = p.locator('.sig-pkg').first()
    await el.scrollIntoViewIfNeeded(); await p.waitForTimeout(700)
    await el.screenshot({path:`${S}/foot${w}.png`})
    const bad = await p.evaluate(() => {
      const out=[]
      document.querySelectorAll('.card-foot .price').forEach(el=>{
        const r=el.getBoundingClientRect()
        // a single-line price is about one line tall; more means it wrapped
        const lh = parseFloat(getComputedStyle(el).fontSize)*1.6
        out.push({text:el.textContent.trim().slice(0,22), h:Math.round(r.height), oneLine: r.height < lh*1.4})
      })
      const overflow = [...document.querySelectorAll('.sig-pkg .card')].some(c=>c.scrollWidth>c.clientWidth+1)
      return {prices:out, horizontalOverflow:overflow}
    })
    console.log(w+'px', JSON.stringify(bad))
    await p.close()
  }
  await b.close()
})()
