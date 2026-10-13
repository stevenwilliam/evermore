const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(900)
  console.log('tuft is a DOM descendant of the lid:',
    await p.evaluate(()=>document.querySelector('#bot-a .bf-lid').contains(document.querySelector('#bot-a .bot-hair'))))
  for (const ry of [-60,-22,20,60,98,140]) {
    await p.evaluate(v=>document.querySelector('#bot-spin-a').style.setProperty('--ry',v+'deg'), ry)
    await p.waitForTimeout(450)
    const r = await p.evaluate(()=>{
      const l=document.querySelector('#bot-a .bf-lid').getBoundingClientRect()
      const h=document.querySelector('#bot-a .bot-hair').getBoundingClientRect()
      return {hw:Math.round(h.width), hh:Math.round(h.height),
              dx:Math.round(h.left+h.width/2-(l.left+l.width/2)),
              dy:Math.round(h.top+h.height/2-(l.top+l.height/2))}
    })
    console.log(`  ry=${String(ry).padStart(4)}  tuft ${String(r.hw).padStart(3)}x${String(r.hh).padStart(3)}  offset from lid centre (${String(r.dx).padStart(4)},${String(r.dy).padStart(4)})`)
  }
  await b.close()
})()
