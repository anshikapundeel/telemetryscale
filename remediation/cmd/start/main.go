// start triggers a remediation. A second start for a service with a running
// workflow is rejected by Temporal (same workflow ID), which prevents duplicate incidents.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"

	rem "github.com/anshikapundeel/telemetryscale/remediation"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

func main() {
	svc := flag.String("service", "loadgen", "service to remediate")
	disruptive := flag.Bool("disruptive", false, "require human approval")
	flag.Parse()
	c, err := client.Dial(client.Options{HostPort: "localhost:7233"})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	run, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{ID: rem.WorkflowID(*svc), TaskQueue: rem.TaskQueue},
		rem.RemediationWorkflow, rem.Incident{Service: *svc, Disruptive: *disruptive})
	var already *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &already) {
		fmt.Println("duplicate incident suppressed: remediation already running for", *svc)
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("started", run.GetID(), run.GetRunID())
	if *disruptive {
		fmt.Println("waiting for approval: temporal workflow signal --workflow-id", rem.WorkflowID(*svc), "--name approval --input true")
	}
}
