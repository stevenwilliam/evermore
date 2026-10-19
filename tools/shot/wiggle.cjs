const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:2})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1400)
  const rail = await p.locator('.weekrail').boundingBox()
  await p.evaluate(y=>window.scrollTo(0,y-120), rail.y)
  await p.waitForTimeout(400)
  const before = await p.evaluate(()=>{
    const ink=document.querySelector('.weekline-ink')
    return {anims: ink.getAnimations().length,
            pol: document.querySelector('.polaroid').getAnimations().length,
            tilt: getComputedStyle(document.querySelector('.weekstrip > .polaroid:nth-child(4)')).rotate,
            hang: getComputedStyle(document.querySelector('.weekstrip > .polaroid:nth-child(4)')).marginTop}
  })
  console.log('at rest ->', JSON.stringify(before))
  // put the pointer on the line itself
  const hit = await p.locator('.weekline-hit').boundingBox()
  await p.mouse.move(hit.x + hit.width*0.5, hit.y + hit.height*0.72)
  await p.waitForTimeout(120)
  const during = await p.evaluate(()=>{
    const ink=document.querySelector('.weekline-ink')
    return {lineAnims: ink.getAnimations().map(a=>a.animationName||a.constructor.name),
            polAnims: document.querySelector('.polaroid').getAnimations().map(a=>a.animationName||a.constructor.name),
            swaying: [...document.querySelectorAll('.polaroid')].filter(e=>e.getAnimations().length>0).length}
  })
  console.log('pointer on the line ->', JSON.stringify(during))
  await p.screenshot({path:S+'/wiggle.png', clip:{x:0,y:Math.max(0,rail.y-150-(rail.y-120)),width:1366,height:300}})
  await b.close()
})()
