// Pomen - plugin for Intermasq
// Copyright (C) 2026 AlexRus1234
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

import { test, expect } from '@playwright/test'

// Минимальный smoke: открывает index.html pomen-ci, проверяет что Vue-апп
// смонтировался и заголовок/футер видны. Это regression на "UI не сломался
// после рефакторинга" (например, после перехода на //go:embed web или правок
// app.js). Расширить: VM CRUD через UI, provision-flow, render domainExample.
test('Pomen UI mounts and shows version', async ({ page }) => {
  await page.goto('/')
  // Vue должен смонтироваться в #app.
  await expect(page.locator('#app')).toBeVisible({ timeout: 10_000 })
  // Заголовок с именем плагина.
  await expect(page.locator('h2')).toContainText('Pomen')
  // Версия должна подтянуться из /api/version (footer появляется только когда
  // pomenVersion !== ''). Ждём footer — это означает что /api/version тоже
  // отработал (т.е. pomen-ci жив и отвечает).
  const footer = page.locator('footer')
  await expect(footer).toBeVisible({ timeout: 10_000 })
  await expect(footer).toContainText(/Pomen v.+/)
})
