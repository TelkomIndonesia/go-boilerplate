package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/telkomindonesia/go-boilerplate/pkg/cmd"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
		<-done
		cancel()
	}()

	c, err := cmd.NewServer()
	if err != nil {
		log.Fatal(err)
	}
	if err = c.Exec(ctx); err != nil {
		log.Println(err)
	}
}
