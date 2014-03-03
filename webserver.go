package main

import (
	"fmt"
	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/context"
	"github.com/stretchr/goweb/responders"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

type Webserver struct {
	Address string
}

func (this *Webserver) init() {
	this.turnOffAutoEnvelop()
	mapRoutes()
	goweb.MapStatic("static/", "static")

	s := &http.Server{
		Addr:           this.Address,
		Handler:        goweb.DefaultHttpHandler(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	listener, listenErr := net.Listen("tcp", this.Address)

	if listenErr != nil {
		log.Fatalf("Could not listen: %s", listenErr)
	}
	fmt.Printf("Webserver listening on %s", this.Address)

	goweb.Map(func(c context.Context) error {
		return goweb.API.Respond(c, 404, nil, []string{"File not found"})
	})

	this.registerShutdownFunction(listener, c)

	log.Fatalf("Error in Serve: %s", s.Serve(listener))
}

func (this *Webserver) turnOffAutoEnvelop() {
	apiResponder := responders.NewGowebAPIResponder(goweb.CodecService, goweb.Respond)
	apiResponder.AlwaysEnvelopResponse = false
	goweb.API = apiResponder
}

func (this *Webserver) registerShutdownFunction(listener net.Listener, c chan os.Signal) {
	go func() {
		for _ = range c {
			log.Print("Stopping the server...")
			listener.Close()

			log.Print("Tearing down...")

			log.Fatal("Finished - bye bye. ;-)")
		}
	}()
}
