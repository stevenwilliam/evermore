const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S=process.argv[2]
  for (const w of [1366, 1024, 820, 720, 390]) {
    const p = await (await b.newContext({viewport:{width:w,height:900}, deviceScaleFactor: w===1366?2:1})).newPage()
    await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1300)
    const r = await p.evaluate(()=>{
      const items=[...document.querySelectorAll('.stat-item')]
      const over=items.filter(i=>{const b=i.querySelector('b'); return b.scrollWidth>i.clientWidth+1}).length
      const cs=getComputedStyle(items[0].querySelector('b'))
      return {size:cs.fontSize, align:getComputedStyle(items[0]).textAlign, overflowing:over,
              widths:items.map(i=>Math.round(i.getBoundingClientRect().width))}
    })
    console.log(`${String(w).padStart(4)}px  number ${r.size}  align ${r.align}  overflowing ${r.overflowing}  item widths ${r.widths.join(',')}`)
    if (w===1366){ const el=await p.locator('.section.tight').boundingBox()
      await p.evaluate(y=>window.scrollTo(0,y-60), el.y); await p.waitForTimeout(300)
      await p.screenshot({path:S+'/stats.png', clip:{x:0,y:0,width:1366,height:240}}) }
    await p.close()
  }
  await b.close()
})()
