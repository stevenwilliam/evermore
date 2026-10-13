const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(600)
  const run = async (label, ms) => {
    const r = await p.evaluate(async (ms) => {
      const lid = document.querySelector('#bot-a .bf-lid')
      const out=[]; const t0=performance.now()
      return await new Promise(res=>{const tick=()=>{const l=lid.getBoundingClientRect()
        out.push([+(l.top+l.height/2).toFixed(2), +(l.left+l.width/2).toFixed(2)])
        if(performance.now()-t0<ms) requestAnimationFrame(tick); else res(out)};tick()})
    }, ms)
    const cy=r.map(x=>x[0]), cx=r.map(x=>x[1])
    const iHi=cy.indexOf(Math.min(...cy)), iLo=cy.indexOf(Math.max(...cy))
    console.log(label)
    console.log('  lid HIGHEST (cy min):  cy', cy[iHi], ' cx', cx[iHi])
    console.log('  lid LOWEST  (cy max):  cy', cy[iLo], ' cx', cx[iLo])
    console.log('  => as the lid rises, x moves', (cx[iHi]-cx[iLo]).toFixed(2), 'px')
    console.log('  amplitudes: y', (Math.max(...cy)-Math.min(...cy)).toFixed(2),
                ' x', (Math.max(...cx)-Math.min(...cx)).toFixed(2))
  }
  await run('CLOSED', 7000)
  await p.locator('#bot-a').click(); await p.waitForTimeout(1600)
  await run('OPEN  ', 6500)
  await b.close()
})()
