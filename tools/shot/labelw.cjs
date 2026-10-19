const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1500)
  console.log(await p.evaluate(()=>{
    const out=[]
    const card=document.querySelector('.daycard')
    out.push('card width '+Math.round(card.getBoundingClientRect().width))
    const body=document.querySelector('.daycard-set-body')
    const cs=getComputedStyle(body)
    out.push('set-body inner width '+Math.round(body.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight))
      +'  padding '+cs.paddingLeft+'/'+cs.paddingRight)
    ;[...document.querySelectorAll('.daycard-set-label')].slice(0,4).forEach(e=>{
      const r=e.getBoundingClientRect()
      const fs=parseFloat(getComputedStyle(e).fontSize)
      // natural width if it could not wrap
      const probe=e.cloneNode(true); probe.style.cssText='position:absolute;white-space:nowrap;visibility:hidden'
      document.body.appendChild(probe)
      out.push(`"${e.textContent.trim()}"  rendered ${Math.round(r.width)}x${Math.round(r.height)}  fontsize ${fs}  needs ${Math.round(probe.getBoundingClientRect().width)}px on one line`)
      probe.remove()
    })
    // same-card duplicate photos?
    const dup=[...document.querySelectorAll('.daycard')].filter(c=>{
      const s=[...c.querySelectorAll('.daycard-set img')].map(i=>i.getAttribute('src'))
      return s.length===2 && s[0]===s[1]}).length
    out.push('cards whose two photos are identical: '+dup)
    return out.join('\n')
  }))
  await b.close()
})()
