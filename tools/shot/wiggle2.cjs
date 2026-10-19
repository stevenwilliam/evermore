const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1400)
  const rail = await p.locator('.weekrail').boundingBox()
  await p.evaluate(y=>window.scrollTo(0,y-140), rail.y)
  await p.waitForTimeout(400)
  // the line is exposed in the gaps between frames — aim at one
  const gap = await p.evaluate(()=>{
    const ps=[...document.querySelectorAll('.polaroid')].map(e=>e.getBoundingClientRect())
    const line=document.querySelector('.weekline-hit').getBoundingClientRect()
    const x=(ps[0].right+ps[1].left)/2
    return {x, y: line.top + line.height*0.35}
  })
  await p.mouse.move(gap.x, gap.y)
  await p.waitForTimeout(150)
  console.log('pointer in the gap on the line ->', await p.evaluate(()=>{
    const ink=document.querySelector('.weekline-ink')
    return JSON.stringify({
      line: ink.getAnimations().map(a=>a.animationName),
      swaying: [...document.querySelectorAll('.polaroid')].filter(e=>e.getAnimations().some(a=>a.animationName==='hangSwing')).length,
      hasSupported: CSS.supports('selector(:has(a))')})
  }))
  await p.screenshot({path:S+'/wiggle.png', clip:{x:0,y:100,width:1366,height:340}})
  await b.close()
})()
