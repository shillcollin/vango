module test-app

go 1.22

require (
	github.com/vango-dev/vango-ui v0.0.0
	github.com/vango-dev/vango/v2 v2.0.0
)

require github.com/gorilla/websocket v1.5.3 // indirect

replace (
	github.com/vango-dev/vango-ui => ../vango-ui
	github.com/vango-dev/vango/v2 => ../vango_v2
)
