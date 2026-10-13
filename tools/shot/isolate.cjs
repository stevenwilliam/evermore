const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(600)
  // Freeze the rig float so only the lid's own bob is left.
  await p.evaluate(() => {
    const st = document.createElement('style')
    st.textContent = '#bot-a .bot-rig, #bot-a .bot-tuft { animation: none !important; }'
    document.head.appendChild(st)
  })
  await p.waitForTimeout(300)
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
    const cy=r.map(x=>x[0]), cx=r.map(x=>x[1]), hy=r.map(x=>x[2]), hx=r.map(x=>x[3])
    const amp=a=>+(Math.max(...a)-Math.min(...a)).toFixed(2)
    const mean=a=>+(a.reduce((s,v)=>s+v,0)/a.length).toFixed(2)
    const iHi=cy.indexOf(Math.min(...cy)), iLo=cy.indexOf(Math.max(...cy))
    console.log(label, '(rig float frozen)')
    console.log('   LID  cy amp', amp(cy), ' cx amp', amp(cx), ' | as lid rises, x moves', (cx[iHi]-cx[iLo]).toFixed(2))
    console.log('   HAIR cy amp', amp(hy), ' cx amp', amp(hx))
    console.log('   mean gap y', (mean(cy)-mean(hy)).toFixed(2), ' x', (mean(cx)-mean(hx)).toFixed(2))
  }
  await run('CLOSED', 6400)
  await p.locator('#bot-a').click(); await p.waitForTimeout(1700)
  await run('OPEN  ', 5600)
  await b.close()
})()
