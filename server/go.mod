module github.com/Homiakus/WebGate/server

go 1.26
toolchain go1.26.6

require (
	github.com/Homiakus/secureaccess v0.4.0
	modernc.org/sqlite v1.57.0
)

replace github.com/Homiakus/secureaccess => ./third_party/secureaccess
