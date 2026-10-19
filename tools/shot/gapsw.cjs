const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  for (const w of [1366, 1024, 390]) {
    const p = await (await b.newContext({viewport:{width:w,height:900}})).newPage()
    await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1500)
    console.log(w+'px', await p.evaluate(()=>{
      const r=e=>e.getBoundingClientRect()
      const hint=document.querySelector('#bot-hint')
      const wEb=document.querySelector('#week .eyebrow')
      const stats=document.querySelector('.section.tight .container')
      const pEb=document.querySelector('.sig-pkg .eyebrow')
      return 'week seam '+Math.round(r(wEb).top-r(hint).bottom)+'px | packages seam '
        +Math.round(r(pEb).top-r(stats).bottom)+'px'
    }))
    await p.close()
  }
  await b.close()
})()
