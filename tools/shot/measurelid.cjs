const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(900)
  const sample = async (label) => {
    const r = await p.evaluate(async () => {
      const lid = document.querySelector('#bot-a .bf-lid')
      const out = []
      const t0 = performance.now()
      return await new Promise(res => {
        const tick = () => {
          const b = lid.getBoundingClientRect()
          out.push([Math.round(performance.now()-t0), +b.top.toFixed(2), +b.left.toFixed(2),
                    +b.height.toFixed(2), +b.width.toFixed(2)])
          if (performance.now()-t0 < 7000) requestAnimationFrame(tick); else res(out)
        }
        tick()
      })
    })
    const tops = r.map(x=>x[1]), lefts = r.map(x=>x[2]), hs = r.map(x=>x[3])
    const f = a => [Math.min(...a).toFixed(2), Math.max(...a).toFixed(2), (Math.max(...a)-Math.min(...a)).toFixed(2)]
    console.log(label, 'top[min,max,amp]', f(tops), 'left', f(lefts), 'height', f(hs))
    // where in the cycle is t=0?
    console.log('   first 6 tops:', tops.slice(0,6).join(' '), ' ... phase at t=0 is', (tops[0]===Math.min(...tops)?'MIN':tops[0]===Math.max(...tops)?'MAX':'mid'))
  }
  await sample('CLOSED')
  await p.locator('#bot-a .bot-stage').first().click()
  await p.waitForTimeout(1800)
  await sample('OPEN  ')
  await b.close()
})()
