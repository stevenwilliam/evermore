const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1200)
  console.log(await p.evaluate(()=>{
    const e=document.querySelector('.daycard-set .visually-hidden')
    const r=e.getBoundingClientRect(); const cs=getComputedStyle(e)
    return `hidden label box ${Math.round(r.width)}x${Math.round(r.height)} clip=${cs.clipPath||cs.clip} pos=${cs.position}`
      + ` | announced text "${e.textContent}" | in a11y tree: ${!e.hasAttribute('aria-hidden')}`
  }))
  await b.close()
})()
