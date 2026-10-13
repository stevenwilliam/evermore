const { chromium } = require('playwright')
const OUT = process.argv[2]
;(async () => {
  const b = await chromium.launch()
  const ctx = await b.newContext({ viewport:{width:1280,height:900} })
  const p = await ctx.newPage()
  await p.goto('http://127.0.0.1:8082/', { waitUntil:'domcontentloaded', timeout:20000 })
  await p.waitForTimeout(1500)
  await p.screenshot({ path: OUT + '/1-closed.png' })

  // drag to rotate
  const box = await p.locator('#bot-stage').boundingBox()
  await p.mouse.move(box.x + box.width/2, box.y + box.height/2)
  await p.mouse.down()
  await p.mouse.move(box.x + box.width/2 + 150, box.y + box.height/2 - 40, { steps: 14 })
  await p.mouse.up()
  await p.waitForTimeout(700)
  await p.screenshot({ path: OUT + '/2-rotated.png' })

  // open: catch the FULL stage, then the scattered one
  await p.locator('#bot-stage').click()
  await p.waitForTimeout(480)
  await p.screenshot({ path: OUT + '/3-full.png' })
  await p.waitForTimeout(1700)
  await p.screenshot({ path: OUT + '/4-scattered.png' })
  console.log('4 frames captured')
  await b.close()
})()
