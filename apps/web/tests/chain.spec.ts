import { expect, test } from '@playwright/test'
import { readFileSync } from 'node:fs'

const evidence = JSON.parse(readFileSync(process.env.POC_CHAIN_REPORT!, 'utf8'))
const unknownId = 'ffffffff-ffff-4fff-8fff-ffffffffffff'

test('reads every Kotlin fixture through real HTTP after collector restart', async ({ page }, testInfo) => {
  const errors: string[] = []
  const writes: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  page.on('console', message => { if (['error', 'warning'].includes(message.type())) errors.push(message.text()) })
  page.on('request', request => { if (request.method() !== 'GET') writes.push(request.method()) })
  await page.goto('/')
  await page.getByLabel('安装 ID（Installation ID）').fill(evidence.installation_id)
  await page.getByRole('button', { name: '读取 / 刷新' }).click()
  await expect(page.getByTestId('summary')).toContainText('25 / 100')
  for (const count of [50, 75, 100]) {
    await page.getByRole('button', { name: '加载下一页' }).click()
    await expect(page.getByTestId('summary')).toContainText(`${count} / 100`)
  }
  await expect(page.getByTestId('event-row')).toHaveCount(100)
  const ids = await page.getByTestId('event-row').locator('code').allTextContents()
  expect(ids).toEqual([...evidence.fixture_ids].sort())
  expect(new Set(ids).size).toBe(100)
  await expect(page.getByRole('button', { name: '加载下一页' })).toHaveCount(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.evaluate(() => window.scrollTo(0, 0))
  await page.screenshot({ path: testInfo.outputPath('synthetic-chain.png') })
  await page.reload()
  await page.getByLabel('安装 ID（Installation ID）').fill(evidence.installation_id)
  await page.getByRole('button', { name: '读取 / 刷新' }).click()
  await expect(page.getByTestId('summary')).toContainText('25 / 100')
  expect(errors).toEqual([])
  expect(writes).toEqual([])
})

test('validates IDs and clears results when switching installations', async ({ page }) => {
  await page.goto('/')
  await page.getByLabel('安装 ID（Installation ID）').fill('invalid')
  await page.getByRole('button', { name: '读取 / 刷新' }).click()
  await expect(page.getByRole('status')).toContainText('规范的小写 UUID')
  await page.getByLabel('安装 ID（Installation ID）').fill(evidence.installation_id)
  await page.getByRole('button', { name: '读取 / 刷新' }).click()
  await expect(page.getByTestId('event-row')).toHaveCount(25)
  await page.getByLabel('安装 ID（Installation ID）').fill(unknownId)
  await expect(page.getByTestId('event-row')).toHaveCount(0)
  await page.getByRole('button', { name: '读取 / 刷新' }).click()
  await expect(page.getByText('此安装暂无已提交的合成事件。')).toBeVisible()
  await expect(page.getByTestId('summary')).toContainText('0 / 0')
})

test('failed reads clear stale data and manual retry recovers', async ({ page }) => {
  await page.goto('/')
  await page.getByLabel('安装 ID（Installation ID）').fill(evidence.installation_id)
  await page.getByRole('button', { name: '读取 / 刷新' }).click()
  await expect(page.getByTestId('event-row')).toHaveCount(25)
  // Explicit browser fault injection; the happy path never mocks the server or its data.
  await page.route('**/poc/v1/events?*', route => route.fulfill({ status: 503, json: { code: 'storage_unavailable' } }))
  await page.getByRole('button', { name: '读取 / 刷新' }).click()
  await expect(page.getByRole('status')).toContainText('读取失败')
  await expect(page.getByTestId('event-row')).toHaveCount(0)
  await page.unroute('**/poc/v1/events?*')
  await page.getByRole('button', { name: '读取 / 刷新' }).click()
  await expect(page.getByTestId('event-row')).toHaveCount(25)
})

test('loading prevents duplicate reads and rejects a foreign-installation response', async ({ page }) => {
  await page.goto('/')
  await page.getByLabel('安装 ID（Installation ID）').fill(evidence.installation_id)
  let release!: () => void
  const gate = new Promise<void>(resolve => { release = resolve })
  await page.route('**/poc/v1/events?*', async route => {
    await gate
    await route.fulfill({ json: { installation_id: unknownId, events: [], total: 0, next_after: null } })
  })
  await page.getByRole('button', { name: '读取 / 刷新' }).click()
  await expect(page.getByRole('button', { name: '读取中…' })).toBeDisabled()
  await expect(page.getByLabel('安装 ID（Installation ID）')).toBeDisabled()
  release()
  await expect(page.getByRole('status')).toContainText('读取失败')
  await expect(page.getByTestId('event-row')).toHaveCount(0)
})
