const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  for (const w of [1366, 1024]) {
    const p = await (await b.newContext({viewport:{width:w,height:900}})).newPage()
    await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1500)
    console.log(w+'px', await p.evaluate(()=>{
      const ys=[...document.querySelectorAll('.daycard')].map(c=>{
        const sets=c.querySelectorAll('.daycard-set')
        return sets.length===2 ? Math.round(sets[1].getBoundingClientRect().top) : null}).filter(Boolean)
      const names=[...document.querySelectorAll('.daycard-set-name')]
      const over=names.filter(n=>n.scrollHeight>n.clientHeight+1).length
      return `dinner-set tops: ${[...new Set(ys)].join(', ')}  (distinct ${new Set(ys).size})`
        +` | names overflowing their box: ${over}`
        +` | card heights ${[...new Set([...document.querySelectorAll('.daycard')].map(c=>Math.round(c.getBoundingClientRect().height)))].join(',')}`
    }))
    await p.close()
  }
  await b.close()
})()
