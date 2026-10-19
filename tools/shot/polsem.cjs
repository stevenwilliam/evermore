const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1300)
  console.log(await p.evaluate(()=>{
    const f=[...document.querySelectorAll('figure.polaroid')]
    const cap=f.filter(x=>x.querySelector('figcaption')).length
    const strip=document.querySelector('.weekstrip')
    const cs=getComputedStyle(strip)
    return `figures ${f.length} with figcaption ${cap}`
      +` | ribbon attachment ${cs.backgroundAttachment} size ${cs.backgroundSize} pos ${cs.backgroundPosition}`
      +` | today clip ${getComputedStyle(document.querySelector('.polaroid.is-today'),'::before').backgroundColor}`
      +` | hidden period labels ${document.querySelectorAll('.daycard-set .visually-hidden').length}`
  }))
  await b.close()
})()
