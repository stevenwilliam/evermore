const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  for (const w of [1920, 1680, 1536, 1440, 1366, 1280, 1180, 1100, 1024, 900, 768]) {
    const p = await (await b.newContext({viewport:{width:w,height:900}})).newPage()
    await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1200)
    const r = await p.evaluate(()=>{
      const tops = sel => [...document.querySelectorAll('.daycard')]
        .map(c=>{const e=c.querySelectorAll('.daycard-set .daycard-media'); return e[sel]?Math.round(e[sel].getBoundingClientRect().top):null})
        .filter(v=>v!==null)
      const a=tops(0), d=tops(1)
      return {lunch:new Set(a).size, dinner:new Set(d).size,
              dvals:[...new Set(d)].sort((x,y)=>x-y), card:Math.round(document.querySelector('.daycard').getBoundingClientRect().width)}
    })
    console.log(`${String(w).padStart(4)}px card ${String(r.card).padStart(3)}  lunch rows ${r.lunch}  dinner rows ${r.dinner}  ${r.dinner>1?'<-- MISALIGNED '+r.dvals.join(','):''}`)
    await p.close()
  }
  await b.close()
})()
