const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const S = process.argv[2]
  const p = await (await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:2})).newPage()
  const grab = async (url, sel, file) => {
    await p.goto('http://127.0.0.1:8082'+url, {waitUntil:'domcontentloaded', timeout:20000})
    await p.waitForTimeout(1800)
    const el = p.locator(sel).first()
    await el.scrollIntoViewIfNeeded(); await p.waitForTimeout(800)
    await el.screenshot({path:S+'/'+file})
  }
  await grab('/', '#week', 'c-week.png')
  await grab('/', '.sig-pkg', 'c-pkg.png')
  await grab('/menu', '.has-sig', 'c-menu.png')
  console.log(await p.evaluate(()=>{
    const c=document.querySelector('.has-sig .card')
    const m=c.querySelector('.meta')
    return 'card bg: '+getComputedStyle(c).backgroundColor+'  meta ink: '+getComputedStyle(m).color
  }))
  await b.close()
})()
