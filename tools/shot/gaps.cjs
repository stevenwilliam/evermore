const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1600)
  console.log(await p.evaluate(()=>{
    const out=[]
    const probe=(label, prevSel, eyebrowSel, sectionSel)=>{
      const prev=document.querySelector(prevSel), eb=document.querySelector(eyebrowSel), sec=document.querySelector(sectionSel)
      const pr=prev.getBoundingClientRect(), er=eb.getBoundingClientRect(), sr=sec.getBoundingClientRect()
      const cs=getComputedStyle(sec)
      out.push(`${label}
   previous content ends   y=${Math.round(pr.bottom)}
   band starts             y=${Math.round(sr.top)}   (gap above band ${Math.round(sr.top-pr.bottom)}px)
   eyebrow top             y=${Math.round(er.top)}   (band padding-top ${cs.paddingTop})
   TOTAL dead space        ${Math.round(er.top-pr.bottom)}px  (~${((er.top-pr.bottom)/37.8).toFixed(1)}cm)`)
    }
    probe('WEEK STRIP', '.bot-hint, #bot-hint, .bot-pair', '#week .eyebrow', '#week')
    probe('PACKAGES',   '.section.tight .container', '.sig-pkg .eyebrow', '.sig-pkg')
    return out.join('\n\n')
  }))
  await b.close()
})()
