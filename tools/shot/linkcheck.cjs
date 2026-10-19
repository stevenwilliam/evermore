const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const ctx = await b.newContext({viewport:{width:1366,height:900}})
  const p = await ctx.newPage()
  const BASE='http://127.0.0.1:8082'
  const pages = ['/','/menu','/packages','/how-it-works','/corporate','/delivery-area',
                 '/app/sign-in','/app/register','/app/cart']
  // one meal page too
  await p.goto(BASE+'/menu',{waitUntil:'domcontentloaded'})
  const meal = await p.evaluate(()=>{const a=document.querySelector('a[href^="/menu/"]'); return a?a.getAttribute('href'):null})
  if (meal) pages.push(meal)
  const found = new Map()   // path -> Set(method)
  for (const page of pages) {
    const r = await p.goto(BASE+page, {waitUntil:'domcontentloaded'})
    if (!r || r.status()>=400) { console.log(`PAGE ${page} -> ${r?r.status():'ERR'}`); continue }
    await p.waitForTimeout(300)
    const links = await p.evaluate(()=>{
      const out=[]
      document.querySelectorAll('a[href]').forEach(a=>{
        const h=a.getAttribute('href')
        if (h && h.startsWith('/')) out.push(['GET',h.split('#')[0]])
      })
      document.querySelectorAll('form[action]').forEach(f=>{
        const a=f.getAttribute('action')
        if (a && a.startsWith('/')) out.push([(f.getAttribute('method')||'get').toUpperCase(), a])
      })
      return out
    })
    for (const [m,h] of links) {
      if (!found.has(h)) found.set(h, new Set())
      found.get(h).add(m)
    }
  }
  console.log(`collected ${found.size} distinct internal targets`)
  const bad=[]
  for (const [path, methods] of found) {
    for (const m of methods) {
      const res = await ctx.request.fetch(BASE+path, {method:m, maxRedirects:0, failOnStatusCode:false,
        data: m==='POST' ? 'probe=1' : undefined,
        headers: m==='POST' ? {'content-type':'application/x-www-form-urlencoded'} : {}})
      const st = res.status()
      if (st===404 || st===405) bad.push(`${m} ${path} -> ${st}`)
    }
  }
  console.log(bad.length ? 'DANGLING:\n  '+bad.join('\n  ') : 'no dangling links or form actions')
  await b.close()
})()
