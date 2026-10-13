const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.evaluate(()=>document.querySelector('#bot-spin-a').style.setProperty('--ry','-22deg'))
  await p.waitForTimeout(900)
  console.log(await p.evaluate(() => {
    const lid=document.querySelector('#bot-a .bf-lid'), h=document.querySelector('#bot-a .bot-hair')
    const ls=getComputedStyle(lid), hs=getComputedStyle(h)
    const lr=lid.getBoundingClientRect(), hr=h.getBoundingClientRect()
    return JSON.stringify({
      lidTransformStyle: ls.transformStyle, lidOverflow: ls.overflow,
      lidBackface: ls.backfaceVisibility, lidFilter: ls.filter, lidOpacity: ls.opacity,
      hairTransform: hs.transform, hairTransformStyle: hs.transformStyle,
      lidRect: {w:Math.round(lr.width), h:Math.round(lr.height)},
      hairRect:{w:Math.round(hr.width), h:Math.round(hr.height),
                dx:Math.round(hr.left+hr.width/2-(lr.left+lr.width/2)),
                dy:Math.round(hr.top+hr.height/2-(lr.top+lr.height/2))},
    },null,1)
  }))
  await b.close()
})()
