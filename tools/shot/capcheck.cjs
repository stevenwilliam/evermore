const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1400)
  console.log(await p.evaluate(()=>{
    return [...document.querySelectorAll('.polaroid')].map((c,i)=>{
      const r=c.getBoundingClientRect()
      const cap=c.querySelector('.polaroid-caption').getBoundingClientRect()
      const film=c.querySelector('.polaroid-film').getBoundingClientRect()
      const names=[...c.querySelectorAll('.daycard-set-name')].map(n=>Math.round(n.getBoundingClientRect().height))
      return `${i} polaroid h=${Math.round(r.height)} film h=${Math.round(film.height)} capTop=${Math.round(cap.top)} names=${names.join('/')}`
    }).join('\n')
  }))
  await b.close()
})()
