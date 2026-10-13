const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const shot = async (ctxOpts, file, sel, url='http://127.0.0.1:8082/') => {
    const p = await (await b.newContext(ctxOpts)).newPage()
    await p.goto(url, {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(2500)
    const el = p.locator(sel).first()
    await el.scrollIntoViewIfNeeded(); await p.waitForTimeout(900)
    await el.screenshot({path: S+'/'+file})
    return p
  }
  const V = {width:1280,height:900}
  await shot({viewport:V}, 'sig-week.png', '#week')
  await shot({viewport:V}, 'sig-pkg.png',  '.has-sig:not(#week)')
  await shot({viewport:V, colorScheme:'dark'}, 'sig-dark.png', '#week')
  // reduced motion: everything must be still
  const p = await (await b.newContext({viewport:V, reducedMotion:'reduce'})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1200)
  const a = await p.evaluate(()=>{const r=document.querySelector('.r1'),f=document.querySelector('.float')
    return [getComputedStyle(r).strokeDashoffset, getComputedStyle(r).animationName,
            getComputedStyle(f).animationName]})
  await p.waitForTimeout(1500)
  const c = await p.evaluate(()=>getComputedStyle(document.querySelector('.r1')).strokeDashoffset)
  console.log('reduced-motion: animationName r1 =', a[1], ', float =', a[2],
              '| dashoffset', a[0], '->', c, a[0]===c ? 'STILL (correct)' : 'STILL MOVING')
  await b.close()
})()
