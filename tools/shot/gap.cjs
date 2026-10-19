const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:1000}})).newPage()
  for (const url of ['/paket','/']) {
    await p.goto('http://127.0.0.1:8082'+url, {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1300)
    console.log(url, await p.evaluate(()=>{
      const r=e=>e.getBoundingClientRect()
      const main=document.querySelector('main')||document.body
      const last=[...document.querySelectorAll('section')].filter(s=>!s.classList.contains('certs')).pop()
      const certs=document.querySelector('.certs'), foot=document.querySelector('.footer')
      const cs=getComputedStyle(certs)
      return `last section ends ${Math.round(r(last).bottom)} | certs ${Math.round(r(certs).top)}..${Math.round(r(certs).bottom)}`
        +` (pad ${cs.paddingTop}/${cs.paddingBottom}) | footer starts ${Math.round(r(foot).top)}`
        +` | gap above certs ${Math.round(r(certs).top-r(last).bottom)} | gap certs->footer ${Math.round(r(foot).top-r(certs).bottom)}`
    }))
  }
  await b.close()
})()
