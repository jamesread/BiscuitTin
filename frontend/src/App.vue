<template>
	<Section v-if="pairing" title="Pair with BiscuitTin">
		<form class="pair" @submit.prevent="pair">
			<p>Scan the QR code on the TV, or enter the pairing code from <strong>More &gt; Settings &gt; Web control</strong>.</p>
			<input v-model="codeInput" autocomplete="off" autocapitalize="characters" placeholder="ABCD-EFGH" aria-label="Pairing code" />
			<button type="submit">Pair</button>
			<p v-if="pairError" class="error">{{ pairError }}</p>
		</form>
	</Section>

	<template v-else>
	<Section :title="state.title || 'BiscuitTin'">
		<p class="status" aria-live="polite">
			<span v-if="error" class="error">{{ error }}</span>
			<template v-else>
				<span>{{ state.hidden ? 'An app is in front' : screenLabel }}</span>
				<span v-if="state.message"> &middot; {{ state.message }}</span>
			</template>
		</p>

		<div class="dpad" role="group" aria-label="Direction pad">
			<button class="up" aria-label="Up" @click="press(Button.UP)">&#9650;</button>
			<button class="left" aria-label="Left" @click="press(Button.LEFT)">&#9664;</button>
			<button class="select" @click="press(Button.SELECT)">OK</button>
			<button class="right" aria-label="Right" @click="press(Button.RIGHT)">&#9654;</button>
			<button class="down" aria-label="Down" @click="press(Button.DOWN)">&#9660;</button>
		</div>
		<button class="back" @click="press(Button.BACK)">Back</button>
		<div class="volume" role="group" aria-label="Volume">
			<button aria-label="Volume down" @click="press(Button.VOLUME_DOWN)">Vol &minus;</button>
			<button aria-label="Volume up" @click="press(Button.VOLUME_UP)">Vol +</button>
		</div>
	</Section>

	<Section title="Launch">
		<div class="tiles">
			<button
				v-for="tile in state.tiles"
				:key="tile.key"
				:class="{ selected: tile.selected }"
				@click="activate(tile.key)"
			>
				<strong>{{ tile.title }}</strong>
				<small v-if="tile.subtitle">{{ tile.subtitle }}</small>
				<small v-if="tile.running">Running</small>
			</button>
		</div>
	</Section>

	<Section title="Diagnostics">
		<details @toggle="loadDiagnostics">
			<summary>Show diagnostics</summary>
			<pre class="diagnostics">{{ diagnostics }}</pre>
			<button @click="copyDiagnostics">{{ copied ? 'Copied' : 'Copy' }}</button>
		</details>
	</Section>
	</template>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import { Button } from './gen/biscuittin/v1/remote_pb.js'
import { toJsonString } from '@bufbuild/protobuf'
import { GetDiagnosticsResponseSchema } from './gen/biscuittin/v1/debug_pb.js'
import { needsCode, remote, saveCode, savedCode } from './client.js'

const state = reactive({ title: '', tiles: [], screen: '', hidden: false, message: '' })
const error = ref('')
const pairing = ref(false)
const codeInput = ref(savedCode())
const pairError = ref('')
const diagnostics = ref('Loading...')
const copied = ref(false)

// failed shows an error, or the pairing form when the code is missing or
// wrong.
function failed(e) {
	if (needsCode(e)) {
		pairing.value = true
		return
	}
	error.value = e.message
}

async function pair() {
	saveCode(codeInput.value)
	try {
		await remote.getState({})
		pairing.value = false
		pairError.value = ''
		refresh()
	} catch (e) {
		pairError.value = needsCode(e) ? 'That code is not right. Check Settings on the TV.' : e.message
	}
}

async function loadDiagnostics(ev) {
	if (!ev.target.open) {
		return
	}
	copied.value = false
	try {
		const resp = await remote.getRemoteDiagnostics({})
		diagnostics.value = toJsonString(GetDiagnosticsResponseSchema, resp.diagnostics, { prettySpaces: 2 })
	} catch (e) {
		diagnostics.value = e.message
		failed(e)
	}
}

// copyDiagnostics uses the clipboard API where the browser allows it. Over
// plain http on a LAN it usually does not, so fall back to a selection.
async function copyDiagnostics() {
	try {
		await navigator.clipboard.writeText(diagnostics.value)
	} catch {
		const area = document.createElement('textarea')
		area.value = diagnostics.value
		document.body.appendChild(area)
		area.select()
		document.execCommand('copy')
		area.remove()
	}
	copied.value = true
}

const screenLabel = computed(() => ({
	home: 'Home',
	more: 'More',
	settings: 'Settings',
	integrations: 'Integrations',
	actions: 'Pinning actions',
	gamepads: 'Input devices diagnostics',
	arrange: 'Arrange tiles',
	diagnostics: 'App diagnostics',
	'diagnostics-menu': 'Diagnostics',
	audio: 'Audio diagnostics',
	sleep: 'Sleep timer',
}[state.screen] || state.screen))

async function refresh() {
	if (pairing.value) {
		return
	}
	try {
		Object.assign(state, await remote.getState({}))
		error.value = ''
	} catch (e) {
		if (needsCode(e)) {
			pairing.value = true
			return
		}
		error.value = 'Not connected: ' + e.message
	}
}

async function press(button) {
	navigator.vibrate?.(15)
	try {
		await remote.pressButton({ button })
	} catch (e) {
		failed(e)
	}
	refresh()
}

async function activate(key) {
	navigator.vibrate?.(15)
	try {
		await remote.activateTile({ key })
	} catch (e) {
		failed(e)
	}
	refresh()
}

let timer
onMounted(() => {
	refresh()
	timer = setInterval(refresh, 1000)
})
onUnmounted(() => clearInterval(timer))
</script>
