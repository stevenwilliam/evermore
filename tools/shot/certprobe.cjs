const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1400)
  console.log(await p.evaluate(()=>{
    const sub=document.querySelector('.certs-sub'), h=document.querySelector('.certs-title')
    const cont=document.querySelector('.certs .container')
    const r=e=>{const b=e.getBoundingClientRect(); return `${Math.round(b.left)}..${Math.round(b.right)} (centre ${Math.round(b.left+b.width/2)})`}
    const cs=getComputedStyle(sub)
    return `page centre 683\n container ${r(cont)}\n title ${r(h)}\n sub ${r(sub)}`
      +`\n sub text-align=${cs.textAlign} maxWidth=${cs.maxWidth} margin=${cs.margin} display=${cs.display}`
  }))
  await b.close()
})()
