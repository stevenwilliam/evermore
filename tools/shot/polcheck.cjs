const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  for (const w of [1366, 1024]) {
    const p = await (await b.newContext({viewport:{width:w,height:900}})).newPage()
    await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1400)
    console.log(w+'px', await p.evaluate(()=>{
      const ps=[...document.querySelectorAll('.polaroid')]
      const tops = i => [...new Set(ps.map(c=>{const m=c.querySelectorAll('.daycard-set .daycard-media'); return m[i]?Math.round(m[i].getBoundingClientRect().top):null}).filter(Boolean))]
      const caps=[...new Set(ps.map(c=>Math.round(c.querySelector('.polaroid-caption').getBoundingClientRect().top)))]
      return `polaroids ${ps.length} width ${Math.round(ps[0].getBoundingClientRect().width)}`
        +` | lunch rows ${tops(0).length} dinner rows ${tops(1).length} caption rows ${caps.length}`
        +` | strip scrolls ${document.querySelector('.weekstrip').scrollWidth > document.querySelector('.weekstrip').clientWidth}`
    }))
    await p.close()
  }
  await b.close()
})()
