const { chromium } = require('playwright')
const fs=require('fs')
;(async () => {
  const S=process.argv[2]
  const ic=JSON.parse(fs.readFileSync(S+'/icons.json','utf8'))
  const b = await chromium.launch()
  const p = await (await b.newContext({viewport:{width:520,height:260}, deviceScaleFactor:2})).newPage()
  await p.setContent(`<body style="margin:0;display:flex;gap:24px;align-items:center;justify-content:center;background:#EBD1C5;font:12px system-ui">
    <div style="text-align:center">
      <div style="width:112px;height:112px;border-radius:50%;background:#C2410C;color:#fff;display:grid;place-items:center;box-shadow:0 0 0 6px #fff">
        <span style="width:68px;height:68px;display:block">${ic.sun}</span></div>
      <div style="margin-top:8px">sun 112px</div>
      <div style="margin-top:14px"><span style="display:inline-grid;place-items:center;width:26px;height:26px;border-radius:50%;background:#C2410C;color:#fff;box-shadow:0 0 0 2px #fff"><span style="width:16px;height:16px;display:block">${ic.sun}</span></span> actual 26px</div>
    </div>
    <div style="text-align:center">
      <div style="width:112px;height:112px;border-radius:50%;background:#2F6690;color:#fff;display:grid;place-items:center;box-shadow:0 0 0 6px #fff">
        <span style="width:68px;height:68px;display:block">${ic.moon}</span></div>
      <div style="margin-top:8px">moon 112px</div>
      <div style="margin-top:14px"><span style="display:inline-grid;place-items:center;width:26px;height:26px;border-radius:50%;background:#2F6690;color:#fff;box-shadow:0 0 0 2px #fff"><span style="width:16px;height:16px;display:block">${ic.moon}</span></span> actual 26px</div>
    </div></body>`)
  await p.waitForTimeout(300)
  await p.screenshot({path:S+'/preview.png'})
  console.log('ok')
  await b.close()
})()
