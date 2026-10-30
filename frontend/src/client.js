import { Code, ConnectError, createClient } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { RemoteService } from './gen/biscuittin/v1/remote_pb.js'

const codeKey = 'biscuittin-code'

// takeCodeFromURL keeps the pairing code from a scanned QR link
// (#code=...), then clears it from the address bar.
function takeCodeFromURL() {
	const code = new URLSearchParams(window.location.hash.slice(1)).get('code')
	if (code) {
		localStorage.setItem(codeKey, code)
		history.replaceState(null, '', window.location.pathname + window.location.search)
	}
}
takeCodeFromURL()

export function savedCode() {
	return localStorage.getItem(codeKey) || ''
}

export function saveCode(code) {
	localStorage.setItem(codeKey, code.trim())
}

export function needsCode(e) {
	return ConnectError.from(e).code === Code.Unauthenticated
}

// Every call carries the pairing code.
const sendCode = (next) => (req) => {
	req.header.set('X-Biscuittin-Code', savedCode())
	return next(req)
}

// The page is served by biscuittin itself, so the API is on the same origin.
export const remote = createClient(
	RemoteService,
	createConnectTransport({ baseUrl: window.location.origin, interceptors: [sendCode] }),
)
