const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(700)
  const run = async (label, ms) => {
    const r = await p.evaluate(async (ms) => {
      const lid=document.querySelector('#bot-a .bf-lid'), h=document.querySelector('#bot-a .bot-hair')
      const out=[]; const t0=performance.now()
      return await new Promise(res=>{const tick=()=>{
        const l=lid.getBoundingClientRect(), g=h.getBoundingClientRect()
        out.push([+(l.top+l.height/2).toFixed(2), +(l.left+l.width/2).toFixed(2),
                  +(g.top+g.height/2).toFixed(2), +(g.left+g.width/2).toFixed(2)])
        if(performance.now()-t0<ms) requestAnimationFrame(tick); else res(out)};tick()})
    }, ms)
    const amp=a=>+(Math.max(...a)-Math.min(...a)).toFixed(2)
    const dy=r.map(x=>x[0]-x[2]), dx=r.map(x=>x[1]-x[3])
    console.log(label)
    console.log('   lid cy amp', amp(r.map(x=>x[0])), '  hair cy amp', amp(r.map(x=>x[2])))
    console.log('   lid cx amp', amp(r.map(x=>x[1])), '  hair cx amp', amp(r.map(x=>x[3])))
    console.log('   GAP lid-hair  y: amp', amp(dy), ' x: amp', amp(dx), '  <- near 0 means attached')
    return {gy:(dy.reduce((s,v)=>s+v,0)/dy.length).toFixed(2), gx:(dx.reduce((s,v)=>s+v,0)/dx.length).toFixed(2)}
  }
  const c = await run('CLOSED', 7000)
  await p.locator('#bot-a').click(); await p.waitForTimeout(1700)
  const o = await run('OPEN  ', 6500)
  console.log('\n  mean gap closed  y', c.gy, ' x', c.gx)
  console.log('  mean gap open    y', o.gy, ' x', o.gx, '  <- same gap in both states means it stayed put on the lid')
  await b.close()
})()
