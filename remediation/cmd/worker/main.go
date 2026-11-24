package main

import (
	"log"

	rem "github.com/anshikapundeel/telemetryscale/remediation"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	c, err := client.Dial(client.Options{HostPort: "localhost:7233"})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	w := worker.New(c, rem.TaskQueue, worker.Options{})
	w.RegisterWorkflow(rem.RemediationWorkflow)
	w.RegisterActivity(&rem.Activities{R: rem.NewMemRemediator()})
	log.Fatal(w.Run(worker.InterruptCh()))
}
