const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(900)
  console.log(await p.evaluate(() => {
    const lid = document.querySelector('#bot-a .bf-lid')
    const cs = getComputedStyle(lid)
    const before = lid.getBoundingClientRect()
    lid.style.animation = 'none'
    lid.style.transform = 'rotateX(-90deg) translateZ(-24px) translateY(200px)'
    const after = lid.getBoundingClientRect()
    return JSON.stringify({
      tag: lid.className,
      animName: cs.animationName,
      computedTransform: cs.transform,
      parentPreserve: getComputedStyle(lid.parentElement).transformStyle,
      parentClass: lid.parentElement.className,
      beforeTop: +before.top.toFixed(2), afterTop: +after.top.toFixed(2),
      afterComputed: getComputedStyle(lid).transform,
    }, null, 1)
  }))
  await b.close()
})()
