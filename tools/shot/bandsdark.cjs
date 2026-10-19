const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  for (const s of ['light','dark']) {
    const p = await (await b.newContext({viewport:{width:1280,height:900}, colorScheme:s})).newPage()
    await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1200)
    console.log(s, await p.evaluate(()=>JSON.stringify({
      week: getComputedStyle(document.querySelector('#week')).backgroundColor,
      pkg:  getComputedStyle(document.querySelector('.band-wash')).backgroundColor,
      stats:getComputedStyle(document.querySelector('.section.tight')).backgroundColor,
      weekPad: getComputedStyle(document.querySelector('#week')).paddingTop
        +'/'+getComputedStyle(document.querySelector('#week')).paddingBottom})))
    await p.close()
  }
  await b.close()
})()
