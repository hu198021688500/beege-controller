package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/hugb/beege-controller/config"
	"github.com/hugb/beege-controller/server"
)

func main() {
	configFile := flag.String("c", "", "Configuration File")
	flag.Parse()

	c := config.DefaultConfig()
	if *configFile != "" {
		c = config.InitConfigFromFile(*configFile)
	}

	exitCh := make(chan error)
	go worker(c, exitCh)

	for {
		err := <-exitCh
		log.Printf("server thread stop by error:%s.\n", err)
		for i := 0; i < 3; i++ {
			log.Printf("restart server after %d seconds.\n", 3-i)
			time.Sleep(1 * time.Second)
		}
		go worker(c, exitCh)
	}
}

func worker(c *config.Config, exitCh chan error) {
	defer func() {
		if err := recover(); err != nil {
			exitCh <- fmt.Errorf("%s", err)
		} else {
			exitCh <- nil
		}
	}()

	server.NewController(c).Start()
}
