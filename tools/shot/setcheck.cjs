const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1500)
  console.log(await p.evaluate(()=>{
    const cards=[...document.querySelectorAll('.daycard')]
    const labelWraps = [...document.querySelectorAll('.daycard-set-label')]
      .filter(e=>e.getBoundingClientRect().height > parseFloat(getComputedStyle(e).fontSize)*1.8).length
    const imgs=[...document.querySelectorAll('.daycard-set img')]
    return `cards ${cards.length} | sets ${document.querySelectorAll('.daycard-set').length}`
      + ` | cards with 2 sets ${cards.filter(c=>c.querySelectorAll('.daycard-set').length===2).length}`
      + ` | photos ${imgs.length}, distinct src ${new Set(imgs.map(i=>i.getAttribute('src'))).size}`
      + ` | labels wrapping ${labelWraps}`
      + ` | broken imgs ${imgs.filter(i=>!i.complete||i.naturalWidth===0).length}`
  }))
  await b.close()
})()
