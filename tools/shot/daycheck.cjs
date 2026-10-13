const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  for (const scheme of ['light','dark']) {
    const p = await (await b.newContext({viewport:{width:1366,height:900}, colorScheme:scheme})).newPage()
    await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1400)
    console.log(scheme, await p.evaluate(()=>{
      const today=document.querySelector('.daycard.is-today'), rest=document.querySelector('.daycard:not(.is-today)')
      const g=e=>getComputedStyle(e)
      return JSON.stringify({
        todayHead:g(today.querySelector('.daycard-head')).backgroundColor,
        restHead:g(rest.querySelector('.daycard-head')).backgroundColor,
        cardBg:g(rest).backgroundColor,
        meta:g(rest.querySelector('.meta')).color,
        date:g(rest.querySelector('.daycard-date')).color,
        todayDate:g(today.querySelector('.daycard-date')).color})
    }))
    await p.close()
  }
  await b.close()
})()
