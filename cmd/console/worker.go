package main

import "github.com/kkz6/launch-go/internal/pkg/console"

// workerCommand starts the background job worker and scheduler.
type workerCommand struct{}

func (workerCommand) Signature() string   { return "queue:work" }
func (workerCommand) Description() string { return "Start the background job worker" }
func (workerCommand) Extend() console.Extend {
	return console.Extend{Category: "queue"}
}

func (workerCommand) Handle(ctx console.Context) error {
	worker, err := bootstrapWorker()
	if err != nil {
		ctx.Error(err.Error())
		return err
	}

	return worker.run(ctx)
}
