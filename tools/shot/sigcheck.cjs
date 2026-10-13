const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1200)
  console.log(await p.evaluate(() => {
    const out = {}
    const stop = document.querySelector('#venA stop')
    out.stopColorComputed = getComputedStyle(stop).stopColor
    out.stopColorAttr = stop.getAttribute('stop-color')
    const sec = document.querySelector('.has-sig')
    const cs = getComputedStyle(sec)
    out.section = {position: cs.position, overflow: cs.overflow}
    out.container = (() => { const c = sec.querySelector(':scope > .container')
      const s = getComputedStyle(c); return {position:s.position, zIndex:s.zIndex} })()
    const svg = sec.querySelector('.sig-ribbon')
    const ss = getComputedStyle(svg)
    out.svg = {position:ss.position, zIndex:ss.zIndex, pointerEvents:ss.pointerEvents,
               w:Math.round(svg.getBoundingClientRect().width), h:Math.round(svg.getBoundingClientRect().height)}
    const r1 = sec.querySelector('.r1')
    out.r1 = {anim:getComputedStyle(r1).animationName, dur:getComputedStyle(r1).animationDuration,
              pathLength:r1.getAttribute('pathLength'), dash:getComputedStyle(r1).strokeDasharray}
    const fl = document.querySelector('.sig-ribbon .float')
    out.float = fl ? getComputedStyle(fl).animationName+' '+getComputedStyle(fl).animationDuration : 'none'
    const card = document.querySelector('.daycard')
    out.cardBg = card ? getComputedStyle(card).backgroundColor : '-'
    return JSON.stringify(out, null, 1)
  }))
  // is the dash offset actually moving?
  const a = await p.evaluate(()=>getComputedStyle(document.querySelector('.r1')).strokeDashoffset)
  await p.waitForTimeout(1500)
  const c = await p.evaluate(()=>getComputedStyle(document.querySelector('.r1')).strokeDashoffset)
  console.log('dashoffset t0', a, '-> t+1.5s', c, a===c ? 'NOT ANIMATING' : 'animating')
  await b.close()
})()
