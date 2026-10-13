const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1280,height:700}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(900)
  const ends = async (label, from, to) => {
    const r = await p.evaluate(([from,to]) => {
      const lid = document.querySelector('#bot-a .bf-lid')
      const prevA = lid.style.animation, prevT = lid.style.transform
      lid.style.animation = 'none'
      const read = tr => { lid.style.transform = tr
        const b = lid.getBoundingClientRect()
        return {cx:+(b.left+b.width/2).toFixed(2), cy:+(b.top+b.height/2).toFixed(2),
                top:+b.top.toFixed(2), w:+b.width.toFixed(2), h:+b.height.toFixed(2)} }
      const a = read(from), z = read(to)
      lid.style.animation = prevA; lid.style.transform = prevT
      return {a,z}
    }, [from,to])
    console.log(label)
    console.log('   from:', JSON.stringify(r.a))
    console.log('   to  :', JSON.stringify(r.z))
    console.log('   delta cx', (r.z.cx-r.a.cx).toFixed(2), ' cy', (r.z.cy-r.a.cy).toFixed(2),
                ' scale', (r.z.w/r.a.w).toFixed(4))
  }
  await ends('lidRest (closed)',
    'rotateX(-90deg) translateZ(-24px) translateY(6px) rotateZ(-1.5deg)',
    'rotateX(-90deg) translateZ(-24px) translateY(30px) rotateZ(1.5deg)')
  await ends('lidBob (open)',
    'rotateX(-90deg) translateZ(-24px) translateY(62px) rotateZ(-3deg)',
    'rotateX(-90deg) translateZ(-24px) translateY(84px) rotateZ(2deg)')
  // and where the hair currently sits, for reference
  const h = await p.evaluate(() => { const b=document.querySelector('#bot-a .bot-hair').getBoundingClientRect()
    return {cx:+(b.left+b.width/2).toFixed(2), bottom:+b.bottom.toFixed(2)} })
  console.log('hair now:', JSON.stringify(h))
  await b.close()
})()
