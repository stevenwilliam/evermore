const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:1000}})).newPage()
  await p.goto('http://127.0.0.1:8082/paket', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1400)
  console.log(await p.evaluate(()=>{
    const cards=[...document.querySelectorAll('.sig-pkg .card')]
    const h=cards.map(c=>Math.round(c.querySelector('h2').getBoundingClientRect().top))
    const lists=cards.map(c=>Math.round(c.querySelector('.perks').getBoundingClientRect().top))
    const btn=cards.map(c=>Math.round(c.querySelector('.btn').getBoundingClientRect().top))
    const perks=cards.map(c=>c.querySelectorAll('.perk').length)
    const off=cards.map(c=>c.querySelectorAll('.perk.is-off').length)
    return `headings ${[...new Set(h)].join(',')} (distinct ${new Set(h).size})`
      +` | perk lists ${[...new Set(lists)].join(',')} (distinct ${new Set(lists).size})`
      +` | buttons distinct ${new Set(btn).size}`
      +` | rows per card ${perks.join('/')} | crosses ${off.join('/')}`
  }))
  await b.close()
})()
