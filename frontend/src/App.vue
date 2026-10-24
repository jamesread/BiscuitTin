<template>
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
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import Section from 'picocrank/vue/components/Section.vue'
import { Button } from './gen/biscuittin/v1/remote_pb.js'
import { remote } from './client.js'

const state = reactive({ title: '', tiles: [], screen: '', hidden: false, message: '' })
const error = ref('')

const screenLabel = computed(() => ({
	home: 'Home',
	more: 'More',
	settings: 'Settings',
	integrations: 'Integrations',
	actions: 'Pinning actions',
	gamepads: 'Gamepad info',
	diagnostics: 'Diagnostics',
}[state.screen] || state.screen))

async function refresh() {
	try {
		Object.assign(state, await remote.getState({}))
		error.value = ''
	} catch (e) {
		error.value = 'Not connected: ' + e.message
	}
}

async function press(button) {
	navigator.vibrate?.(15)
	try {
		await remote.pressButton({ button })
	} catch (e) {
		error.value = e.message
	}
	refresh()
}

async function activate(key) {
	navigator.vibrate?.(15)
	try {
		await remote.activateTile({ key })
	} catch (e) {
		error.value = e.message
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
