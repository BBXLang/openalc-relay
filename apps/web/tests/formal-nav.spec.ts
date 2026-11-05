import { expect, test } from '@playwright/test'

/*
 * 正式 UI 导航 e2e（FORMAL-UI-MAIL-PLAN §2.1 / §2.2）。
 * 运行方式与既有 chain.spec 相同：需要 POC_BASE_URL 指向构建产物所在站点，
 * 例如 `npx vite preview` 或本机 poc-collector -web-dir dist。
 * 正式列表数据未接通，本文件只验证导航、状态文案、草稿与主题行为，不伪造会话数据。
 */

const DRAFT_SMS = '正式 UI 草稿：短信正文样例'
const DRAFT_MAIL = '正式 UI 草稿：邮件正文样例'

test('root redirects to /sms and top tabs switch channels keeping URL', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveURL(/#\/sms$/)
  await expect(page.getByRole('tab', { name: '短信' })).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByRole('tabpanel', { name: '短信' })).toBeVisible()
  await expect(page.getByRole('tabpanel', { name: '短信' }).getByText('正式同步未接通')).toBeVisible()

  await page.getByRole('tab', { name: '邮件' }).click()
  await expect(page).toHaveURL(/#\/mail$/)
  await expect(page.getByRole('tab', { name: '邮件' })).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByRole('tabpanel', { name: '邮件' }).getByText('正式同步未接通')).toBeVisible()
})

test('channel drafts are stored per channel and survive tab switching and reload', async ({ page }) => {
  await page.goto('/#/sms')
  await page.getByLabel('正文').fill(DRAFT_SMS)
  await page.getByRole('tab', { name: '邮件' }).click()
  await expect(page).toHaveURL(/#\/mail$/)
  await page.getByLabel('正文').fill(DRAFT_MAIL)

  // 切回短信：邮件草稿不得覆盖短信草稿。
  await page.getByRole('tab', { name: '短信' }).click()
  await expect(page).toHaveURL(/#\/sms$/)
  await expect(page.getByLabel('正文')).toHaveValue(DRAFT_SMS)

  await page.getByRole('tab', { name: '邮件' }).click()
  await expect(page.getByLabel('正文')).toHaveValue(DRAFT_MAIL)

  // 刷新后草稿元数据仍恢复（明文本地临时草稿）。
  await page.reload()
  await expect(page).toHaveURL(/#\/mail$/)
  await expect(page.getByLabel('正文')).toHaveValue(DRAFT_MAIL)
  await page.getByRole('tab', { name: '短信' }).click()
  await expect(page.getByLabel('正文')).toHaveValue(DRAFT_SMS)
})

test('sidebar entries reach their routes and every close path preserves drafts', async ({ page }) => {
  await page.goto('/#/sms')
  await page.getByLabel('正文').fill(DRAFT_SMS)

  const menu = page.getByRole('button', { name: '打开侧栏菜单' })
  const sidebar = page.getByRole('dialog', { name: '侧栏菜单' })

  await menu.click()
  await expect(sidebar).toBeVisible()
  await expect(menu).toHaveAttribute('aria-expanded', 'true')

  // 配置入口 → /settings
  await sidebar.getByRole('link', { name: '配置', exact: true }).click()
  await expect(page).toHaveURL(/#\/settings$/)
  await expect(sidebar).toBeHidden()

  // 邮件组 → 添加账号 → /mail/accounts
  await menu.click()
  await sidebar.getByRole('button', { name: '邮件' }).click()
  await sidebar.getByRole('link', { name: '添加账号' }).click()
  await expect(page).toHaveURL(/#\/mail\/accounts$/)

  // 设备组 → 配对设备 → /settings；空态文案
  await menu.click()
  await sidebar.getByRole('button', { name: '设备' }).click()
  await expect(sidebar.getByText('未配对设备')).toBeVisible()
  await sidebar.getByRole('link', { name: '配对设备' }).click()
  await expect(page).toHaveURL(/#\/settings$/)

  // 遮罩关闭：草稿与 URL 保持不变
  await page.getByRole('button', { name: '打开侧栏菜单' }).click()
  await expect(sidebar).toBeVisible()
  await page.locator('.shell-scrim').click({ position: { x: 375, y: 250 } })
  await expect(sidebar).toBeHidden()
  await expect(page).toHaveURL(/#\/settings$/)
  await expect(menu).toHaveAttribute('aria-expanded', 'false')

  // 关闭钮关闭
  await menu.click()
  await sidebar.getByRole('button', { name: '关闭侧栏' }).click()
  await expect(sidebar).toBeHidden()

  // 浏览器返回关闭（不改变 URL）
  await menu.click()
  await expect(sidebar).toBeVisible()
  await page.goBack()
  await expect(sidebar).toBeHidden()
  await expect(page).toHaveURL(/#\/settings$/)

  // 关闭过程不破坏草稿：回到短信页草稿仍在
  await page.getByRole('tab', { name: '短信' }).click()
  await expect(page.getByLabel('正文')).toHaveValue(DRAFT_SMS)
})

test('detail routes are reachable and return to their lists', async ({ page }) => {
  await page.goto('/#/sms/00000000-0000-4000-8000-000000000000')
  await expect(page.getByText('记录不存在或未同步')).toBeVisible()
  await page.getByRole('link', { name: '返回短信列表' }).click()
  await expect(page).toHaveURL(/#\/sms$/)

  await page.goto('/#/mail/does-not-exist')
  await expect(page.getByText('记录不存在或未同步')).toBeVisible()
  await page.getByRole('link', { name: '返回邮件列表' }).click()
  await expect(page).toHaveURL(/#\/mail$/)

  // 设备详情同样有独立 URL 与未同步态
  await page.goto('/#/devices/unknown-device')
  await expect(page.getByText('记录不存在或未同步')).toBeVisible()
})

test('send buttons stay disabled with the not-connected reason visible', async ({ page }) => {
  await page.goto('/#/sms')
  const smsSend = page.getByRole('button', { name: /发送：正式链路未接通/ })
  await expect(smsSend).toBeDisabled()
  await expect(page.getByText(/发送不可用：正式链路未接通/).first()).toBeVisible()

  await page.getByRole('tab', { name: '邮件' }).click()
  const mailSend = page.getByRole('button', { name: /发送：正式链路未接通/ })
  await expect(mailSend).toBeDisabled()
  await expect(page.getByText(/发送不可用：正式链路未接通/).first()).toBeVisible()
})

test('settings theme override applies instantly and persists after reload', async ({ page }) => {
  await page.goto('/#/settings')

  const html = page.locator('html')
  await page.getByTestId('theme-light').click()
  await expect(html).toHaveAttribute('data-theme', 'light')
  const lightBg = await html.evaluate(node => getComputedStyle(node).getPropertyValue('--bg').trim())
  expect(lightBg.toLowerCase()).toBe('#fafafa')

  await page.getByTestId('theme-dark').click()
  await expect(html).toHaveAttribute('data-theme', 'dark')
  const darkBg = await html.evaluate(node => getComputedStyle(node).getPropertyValue('--bg').trim())
  expect(darkBg.toLowerCase()).toBe('#111214')

  const themeColor = await page.locator('meta[name="theme-color"]').getAttribute('content')
  expect(themeColor?.toLowerCase()).toBe('#111214')

  await page.reload()
  await expect(html).toHaveAttribute('data-theme', 'dark')

  await page.getByTestId('theme-system').click()
  await expect(html).toHaveAttribute('data-theme', /^(light|dark)$/)
})

test('dev synthetic entry is absent from formal navigation and root never hits collector', async ({ page }) => {
  await page.goto('/#/sms')
  const menu = page.getByRole('button', { name: '打开侧栏菜单' })
  const sidebar = page.getByRole('dialog', { name: '侧栏菜单' })
  await menu.click()
  await expect(sidebar).toBeVisible()
  await expect(sidebar.getByText('合成验证')).toHaveCount(0)
  await expect(sidebar.locator('a[href*="dev/synthetic"]')).toHaveCount(0)
  await expect(page.getByLabel('安装 ID（Installation ID）')).toHaveCount(0)
  await expect(page.getByRole('button', { name: '读取 / 刷新' })).toHaveCount(0)
  await expect(page.getByText('正式同步未接通').first()).toBeVisible()

  await page.goto('/#/mail')
  await expect(page.getByLabel('安装 ID（Installation ID）')).toHaveCount(0)
})

test('horizontal swipe on the list area switches channels', async ({ page }) => {
  await page.goto('/#/sms')
  const pane = page.getByRole('tabpanel', { name: '短信' })
  await expect(pane).toBeAttached()

  // 向左滑：短信 → 邮件
  await page.mouse.move(200, 300)
  await page.mouse.down()
  await page.mouse.move(140, 300, { steps: 4 })
  await page.mouse.move(60, 300, { steps: 4 })
  await page.mouse.up()
  await expect(page).toHaveURL(/#\/mail$/)
  await expect(page.getByRole('tab', { name: '邮件' })).toHaveAttribute('aria-selected', 'true')

  await page.waitForTimeout(400)

  // 向右滑：邮件 → 短信（起点保持在列表内容区内）
  await page.mouse.move(200, 300)
  await page.mouse.down()
  await page.mouse.move(270, 300, { steps: 4 })
  await page.mouse.move(340, 300, { steps: 4 })
  await page.mouse.up()
  await expect(page).toHaveURL(/#\/sms$/)
  await expect(page.getByRole('tab', { name: '短信' })).toHaveAttribute('aria-selected', 'true')
})

test('reduced motion disables the swipe gesture while tab clicks still switch', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/#/sms')

  // 手势被禁用：拖动不切换频道
  await page.mouse.move(200, 300)
  await page.mouse.down()
  await page.mouse.move(60, 300, { steps: 6 })
  await page.mouse.up()
  await expect(page).toHaveURL(/#\/sms$/)

  // 标签点击仍可切换（取消位移动画，不取消切换）
  await page.getByRole('tab', { name: '邮件' }).click()
  await expect(page).toHaveURL(/#\/mail$/)
})
