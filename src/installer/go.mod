module openflux.local/server/installer

go 1.26.9

require (
	golang.org/x/sys v0.48.0
	openflux.local/server/panel v0.0.0
)

require (
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace openflux.local/server/panel => ../web

replace github.com/p1neappleXpress/OpenFlux => ../core
