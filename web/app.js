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

// app.js — фронтенд Pomen (Vue 3 + Bootstrap 5 через CDN).
// Вынесен из index.html, чтобы разнести разметку и логику; embed'ится в
// бинарь через //go:embed web (см. main.go) и раздаётся по /app.js.

const { createApp, ref, computed, onMounted } = Vue

// pluginBase вычисляется один раз при загрузке:
//   • production — Pomen крутится в iframe внутри панели Intermasq.
//     API нужно дёргать через reverse-proxy панели: <parent origin>/plugins/pomen.
//   • dev (POMEN_DEV_PORT) — index.html открыт directly, без parent.
//     pluginBase пустой → axios ходит по относительным путям того же origin.
//   • cross-origin parent или blob-iframe — window.parent.location недоступен
//     (SecurityError). Считаем это dev-сценарием и используем тот же origin.
function getPluginBase() {
    try {
        const parentOrigin = window.parent.location.origin
        // 'null' origin бывает в sandboxed/popup iframe — игнорируем.
        if (parentOrigin && parentOrigin !== 'null' && parentOrigin !== window.location.origin) {
            return `${parentOrigin}/plugins/pomen`
        }
    } catch (_) {
        // cross-origin parent: доступ к location запрещён → не вкладка панели.
    }
    return ''
}

// getAuthToken — JWT панели Intermasq из localStorage. В iframe parent
// первым (production), затем текущий document (dev/E2E). На любом
// SecurityError fallback на пустую строку → axios шлёт без Authorization.
function getAuthToken() {
    try { return window.parent.localStorage.getItem('token') || '' } catch (_) {}
    try { return localStorage.getItem('token') || '' } catch (_) { return '' }
}

const api = axios.create({ baseURL: getPluginBase() })
api.interceptors.request.use(config => {
    const token = getAuthToken()
    if (token) config.headers.Authorization = `Bearer ${token}`
    return config
})

createApp({
    setup() {
        const tab = ref('vms')
        const vms = ref([])
        const state = ref([])
        const containers = ref([])
        const selectedVM = ref('')
        const provisioning = ref('')
        const replaying = ref(false)
        const nodeNames = ref([])
        const newVM = ref({ name: '', node: '', ip: '', webhook_url: '', secret: '' })

        const canAddVM = computed(() =>
            newVM.value.name && newVM.value.node && newVM.value.ip && newVM.value.webhook_url
        )
        const domainExample = computed(() => {
            const vm = selectedVM.value || '<vm>'
            const found = vms.value.find(x => x.name === vm)
            const node = found?.node || '<node>'
            return `<name>.${vm}.${node}.internal`
        })

        async function loadVMs() {
            try {
                const res = await api.get('/api/vms')
                vms.value = res.data || []
            } catch(e) { alert(e.response?.data?.error || "Ошибка") }
        }

        async function addVM() {
            try {
                await api.post('/api/vms', newVM.value)
                newVM.value = { name: '', node: '', ip: '', webhook_url: '', secret: '' }
                await loadVMs()
            } catch(e) { alert(e.response?.data?.error || "Ошибка") }
        }

        async function deleteVM(name) {
            if (!confirm(`Удалить ВМ «${name}» из реестра? Маршруты останутся (offline).`)) return
            try {
                await api.delete('/api/vms/' + encodeURIComponent(name))
                await loadVMs()
            } catch(e) { alert(e.response?.data?.error || "Ошибка") }
        }

        async function loadContainers() {
            if (!selectedVM.value) return
            try {
                const res = await api.get('/api/containers', { params: { vm: selectedVM.value } })
                containers.value = res.data || []
            } catch(e) { alert(e.response?.data?.error || "Ошибка") }
        }

        async function provision(c) {
            provisioning.value = c.real_name
            try {
                const res = await api.post('/api/provision', {
                    vm: selectedVM.value,
                    container_name: c.name || c.real_name
                })
                alert(res.data.message)
            } catch(e) { alert(e.response?.data?.error || "Ошибка") }
            finally { provisioning.value = '' }
        }

        async function loadState() {
            try {
                const res = await api.get('/api/state')
                state.value = res.data || []
            } catch(e) { alert(e.response?.data?.error || "Ошибка") }
        }

        async function deprovision(routeID) {
            if (!confirm(`Удалить маршрут ${routeID}?`)) return
            try {
                await api.delete('/api/deprovision/' + encodeURIComponent(routeID))
                await loadState()
            } catch(e) { alert(e.response?.data?.error || "Ошибка") }
        }

        async function replayCaddy() {
            if (!confirm("Перезаписать конфиг Caddy из таблицы? Перезапустит Caddy на затронутых нодах.")) return
            replaying.value = true
            try {
                const res = await api.post('/api/replay')
                const d = res.data
                let msg = `Восстановлено: ${d.ok}`
                if (d.errors && d.errors.length) {
                    msg += `\nОшибки (${d.errors.length}):\n` + d.errors.join('\n')
                }
                alert(msg)
            } catch(e) { alert(e.response?.data?.error || "Ошибка") }
            finally { replaying.value = false }
        }

        async function loadNodes() {
            try {
                const res = await api.get('/api/nodes')
                nodeNames.value = res.data || []
            } catch(e) { console.error('nodes недоступен', e) }
        }

        onMounted(() => {
            loadVMs()
            loadState()
            loadNodes()
        })

        return {
            tab, vms, state, containers, selectedVM, provisioning, replaying,
            nodeNames, newVM, canAddVM, domainExample,
            loadVMs, addVM, deleteVM, loadContainers, provision, loadState, deprovision, replayCaddy
        }
    }
}).mount('#app')
