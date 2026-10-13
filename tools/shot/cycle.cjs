const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(600)
  const run = async (label, ms) => {
    const r = await p.evaluate(async (ms) => {
      const lid = document.querySelector('#bot-a .bf-lid')
      const hair = document.querySelector('#bot-a .bot-hair')
      const out = []; const t0 = performance.now()
      return await new Promise(res => { const tick = () => {
        const l = lid.getBoundingClientRect(), h = hair.getBoundingClientRect()
        out.push([+(l.top+l.height/2).toFixed(2), +(l.left+l.width/2).toFixed(2), +h.bottom.toFixed(2)])
        if (performance.now()-t0 < ms) requestAnimationFrame(tick); else res(out) }; tick() })
    }, ms)
    const cy=r.map(x=>x[0]), cx=r.map(x=>x[1]), hb=r.map(x=>x[2])
    const st=a=>({min:+Math.min(...a).toFixed(2),max:+Math.max(...a).toFixed(2),
                  amp:+(Math.max(...a)-Math.min(...a)).toFixed(2),
                  mean:+(a.reduce((s,v)=>s+v,0)/a.length).toFixed(2)})
    console.log(label, '\n  lid centre y', JSON.stringify(st(cy)), '\n  lid centre x', JSON.stringify(st(cx)),
                '\n  hair bottom ', JSON.stringify(st(hb)))
    return {cy:st(cy), hb:st(hb)}
  }
  const closed = await run('CLOSED', 7000)
  await p.locator('#bot-a').click()
  await p.waitForTimeout(1600)
  const open = await run('OPEN  ', 6500)
  console.log('\n  lid centre rises on open by', (closed.cy.mean-open.cy.mean).toFixed(2), 'px')
  console.log('  hair rises on open by     ', (closed.hb.mean-open.hb.mean).toFixed(2), 'px')
  console.log('  gap hair-bottom to lid-centre: closed',
    (closed.cy.mean-closed.hb.mean).toFixed(2), ' open', (open.cy.mean-open.hb.mean).toFixed(2))
  await b.close()
})()
