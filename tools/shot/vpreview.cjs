const { chromium } = require('playwright'); const fs=require('fs')
;(async () => {
  const S=process.argv[2]; const v=JSON.parse(fs.readFileSync(S+'/variants.json','utf8'))
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:560,height:190}, deviceScaleFactor:2})).newPage()
  const cell = k => `<div style="text-align:center"><div style="width:96px;height:96px;border-radius:50%;background:#2F6690;color:#fff;display:grid;place-items:center;box-shadow:0 0 0 5px #fff"><span style="width:60px;height:60px;display:block">${v[k]}</span></div><div style="margin-top:6px;font:12px system-ui">flags ${k}</div></div>`
  await p.setContent(`<body style="margin:0;display:flex;gap:20px;align-items:center;justify-content:center;background:#EBD1C5">${Object.keys(v).map(cell).join('')}</body>`)
  await p.waitForTimeout(250)
  await p.screenshot({path:S+'/variants.png'}); console.log('ok'); await b.close()
})()
