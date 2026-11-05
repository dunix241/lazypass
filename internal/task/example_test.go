package task_test

import (
	"context"
	"fmt"

	"lazypass/internal/task"
)

func ExampleScheduler_OnReady() {
	jobs := task.New(context.Background(), task.Options{})
	defer jobs.Close()
	handle, err := jobs.OnReady(task.Task{
		ID: "prepare",
		Run: func(context.Context) error {
			fmt.Println("ready")
			return nil
		},
	})
	if err != nil {
		panic(err)
	}
	jobs.Ready()
	if err := handle.Wait(context.Background()); err != nil {
		panic(err)
	}
	// Output: ready
}
