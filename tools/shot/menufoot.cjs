const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/menu', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1400)
  console.log(await p.evaluate(()=>{
    const f=document.querySelector('.card-foot')
    const pr=f.querySelector('.price').getBoundingClientRect()
    const bt=f.querySelector('.btn').getBoundingClientRect()
    return 'menu foot: price h='+Math.round(pr.height)+' btn left of price? '+(bt.left<pr.left)+
           ' | side-by-side: '+(Math.abs(bt.top-pr.top)<40)
  }))
  await b.close()
})()
