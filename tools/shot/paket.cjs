const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/paket', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1500)
  const el = p.locator('.sig-pkg').first()
  await el.screenshot({path:process.argv[2]+'/paket.png'})
  console.log(await p.evaluate(()=>{
    const r=[]; document.querySelectorAll('.card-foot .price').forEach(e=>{
      const b=e.getBoundingClientRect()
      r.push(e.textContent.trim().split('\n')[0].slice(0,16)+' h='+Math.round(b.height))})
    return r.join(' | ')+'  overflow:'+[...document.querySelectorAll('.card')].some(c=>c.scrollWidth>c.clientWidth+1)
  }))
  await b.close()
})()
