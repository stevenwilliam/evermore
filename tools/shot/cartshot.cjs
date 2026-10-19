const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const ctx = await b.newContext({viewport:{width:1366,height:900}, deviceScaleFactor:2})
  const p = await ctx.newPage()
  await ctx.request.post('http://127.0.0.1:8082/api/v1/auth/login',
    {data:{email:'sinta@example.com',password:'Evermore#2026'}})
  const r = await p.goto('http://127.0.0.1:8082/app/cart', {waitUntil:'domcontentloaded'})
  await p.waitForTimeout(800)
  const el = await p.locator('.section').first().boundingBox()
  await p.screenshot({path:process.argv[2]+'/cart.png', clip:{x:0,y:0,width:1366,height:Math.min(620,el.height+120)}})
  console.log('cart', r.status(), 'rupiah on page:', await p.evaluate(()=>/Rp\s?[0-9]/.test(document.body.innerText)))
  await b.close()
})()
