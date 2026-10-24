import { createClient } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { RemoteService } from './gen/biscuittin/v1/remote_pb.js'

// The page is served by biscuittin itself, so the API is on the same origin.
export const remote = createClient(
	RemoteService,
	createConnectTransport({ baseUrl: window.location.origin }),
)
