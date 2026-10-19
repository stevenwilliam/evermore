const { chromium } = require('playwright')
;(async () => {
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:1366,height:900}})).newPage()
  await p.goto('http://127.0.0.1:8082/', {waitUntil:'domcontentloaded', timeout:20000})
  await p.waitForTimeout(1600)
  console.log(await p.evaluate(()=>{
    const r=e=>e.getBoundingClientRect()
    const cs=e=>getComputedStyle(e)
    const hero=document.querySelector('.hero-full')
    const hint=document.querySelector('#bot-hint')||[...hero.querySelectorAll('*')].pop()
    const week=document.querySelector('#week')
    const wHead=document.querySelector('#week .section-head')
    const wEb=document.querySelector('#week .eyebrow')
    const stats=document.querySelector('.section.tight')
    const pkg=document.querySelector('.sig-pkg')
    const pHead=document.querySelector('.sig-pkg .section-head')
    const pEb=document.querySelector('.sig-pkg .eyebrow')
    return JSON.stringify({
      hero:{padBottom:cs(hero).paddingBottom, bottom:Math.round(r(hero).bottom)},
      hint:{bottom:Math.round(r(hint).bottom), cls:hint.className||hint.id},
      week:{top:Math.round(r(week).top), padTop:cs(week).paddingTop},
      weekHead:{top:Math.round(r(wHead).top), marginTop:cs(wHead).marginTop, marginBottom:cs(wHead).marginBottom},
      weekEyebrow:{top:Math.round(r(wEb).top), marginBottom:cs(wEb).marginBottom},
      stats:{bottom:Math.round(r(stats).bottom), padBottom:cs(stats).paddingBottom},
      pkg:{top:Math.round(r(pkg).top), padTop:cs(pkg).paddingTop},
      pkgHead:{top:Math.round(r(pHead).top)},
      pkgEyebrow:{top:Math.round(r(pEb).top)},
    },null,1)
  }))
  await b.close()
})()
