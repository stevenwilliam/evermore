const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(600)
  const el = p.locator('#week').first()
  await el.scrollIntoViewIfNeeded()
  // restart the animation so the capture times are predictable
  await p.evaluate(()=>{document.querySelectorAll('#week .r1,#week .r2').forEach(e=>{
    e.style.animation='none'; void e.getBoundingClientRect(); e.style.animation=''})})
  for (const [t,name] of [[1500,'w1'],[4000,'w2'],[9000,'w3']]) {
    await p.waitForTimeout(t===1500?1500:(t===4000?2500:5000))
    await el.screenshot({path:`${S}/sig-${name}.png`})
    const o = await p.evaluate(()=>getComputedStyle(document.querySelector('#week .r1')).strokeDashoffset)
    console.log(name, 'at ~'+t+'ms  dashoffset', o)
  }
  await b.close()
})()
