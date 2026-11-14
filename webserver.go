package main

import (
	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/handlers"
	"github.com/stretchr/goweb/responders"
)

// legacyHandler installs the original OneTDraw solver REST routes on the goweb
// default handler and returns it. The routes stay available at their historical
// paths (/puzzles, /puzzle/...) so anything that already talks to them keeps
// working, while the game itself is served by the GraphQL API and SPA.
//
// The solver's own UI lives in legacy-static/ rather than static/ so the
// SvelteKit build (which wipes and fills static/) can never clobber it.
func legacyHandler() *handlers.HttpHandler {
	turnOffAutoEnvelop()
	mapRoutes()
	goweb.MapStatic("legacy-static/", "legacy-static")
	return goweb.DefaultHttpHandler()
}

// turnOffAutoEnvelop stops goweb wrapping every JSON response in an envelope
// object, which would break clients decoding the handler's own structs.
func turnOffAutoEnvelop() {
	apiResponder := responders.NewGowebAPIResponder(goweb.CodecService, goweb.Respond)
	apiResponder.AlwaysEnvelopResponse = false
	goweb.API = apiResponder
}
