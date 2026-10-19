const { chromium } = require('playwright')
const B = 'http://127.0.0.1:8082'
const PUBLIC = ['/','/menu','/packages','/how-it-works','/corporate','/delivery-area','/robots.txt','/sitemap.xml','/nope-404']
const APP    = ['/app/sign-in','/app/register','/app/cart','/app/checkout','/app/orders','/app/packages','/app/addresses','/app/allergies']
const BO     = ['/bo','/bo/menu','/bo/payments']

// What each path should answer, per role. A 303 to sign-in, a 403 for a
// customer on the back office and a 404 on a missing page are all CORRECT
// answers — treating them as failures is what made the first run noisy.
function expectedFor(role, path) {
  if (path === '/nope-404') return [404]
  if (path.startsWith('/bo')) return role === 'admin' ? [200] : role === 'anon' ? [303] : [403]
  const customerOnly = ['/app/checkout','/app/orders','/app/packages','/app/addresses','/app/allergies']
  if (customerOnly.includes(path)) {
    if (role === 'anon') return [303]            // off to sign in
    if (role === 'admin') return [403]           // staff: readable page, not a sign-in loop (D87)
    if (path === '/app/checkout') return [200, 303]  // 303 when the basket is empty
    return [200]
  }
  return [200]
}

;(async () => {
  const b = await chromium.launch()
  const bad = []
  const run = async (role, ctx, paths) => {
    const p = await ctx.newPage()
    for (const path of paths) {
      // Status WITHOUT following redirects, so a 303 reads as a 303.
      const res = await ctx.request.fetch(B + path, { maxRedirects: 0, failOnStatusCode: false })
      const st = res.status()

      // Page errors only: scripts that threw, assets that failed. The console
      // "Failed to load resource" line only echoes the status above.
      const errs = []
      const onErr = e => errs.push('JS ' + e.message)
      const onMsg = m => { const t = m.text()
        if (m.type() === 'error' && !/Failed to load resource/.test(t)) errs.push('console ' + t.slice(0, 90)) }
      p.on('pageerror', onErr); p.on('console', onMsg)
      await p.goto(B + path, { waitUntil: 'domcontentloaded' }).catch(() => {})
      await p.waitForTimeout(220)
      const body = await p.evaluate(() => document.body ? document.body.innerText.slice(0, 300) : '')
      p.off('pageerror', onErr); p.off('console', onMsg)

      const rawJSON = body.trim().startsWith('{"error"')
      const ok = expectedFor(role, path).includes(st) && !rawJSON && errs.length === 0
      if (!ok) bad.push(`${role} ${path} -> ${st}${rawJSON ? ' RAW JSON' : ''}${errs.length ? ' ' + errs[0] : ''}`)
      console.log(`  ${ok ? 'ok  ' : 'FAIL'} ${role.padEnd(8)} ${path.padEnd(17)} ${st}${rawJSON ? '  <-- raw JSON at a browser' : ''}${errs.length ? '  <-- ' + errs[0] : ''}`)
    }
    await p.close()
  }

  const anon = await b.newContext({ viewport: { width: 1366, height: 900 } })
  await run('anon', anon, [...PUBLIC, ...APP, ...BO])
  const cust = await b.newContext({ viewport: { width: 1366, height: 900 } })
  await cust.request.post(B + '/api/v1/auth/login', { data: { email: 'sinta@example.com', password: 'Evermore#2026' } })
  await run('customer', cust, [...PUBLIC, ...APP, ...BO])
  const admin = await b.newContext({ viewport: { width: 1366, height: 900 } })
  await admin.request.post(B + '/api/v1/auth/login', { data: { email: 'admin@evermore.co.id', password: 'Evermore#2026' } })
  await run('admin', admin, [...PUBLIC, ...APP, ...BO])

  console.log(bad.length ? `\n${bad.length} UNEXPECTED:\n  ` + bad.join('\n  ') : '\nno unexpected results across 60 page visits')
  await b.close()
})()
