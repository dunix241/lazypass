package vault

import "context"

type PromptRunner func(func(context.Context) error) error

type OperationRunner func(func(context.Context) error, func(error))

func (run OperationRunner) Execute(prompt PromptRunner, work func(context.Context) error, done func(error)) {
	if run != nil {
		run(work, done)
		return
	}
	done(prompt.Execute(work))
}

func (run PromptRunner) Execute(fn func(context.Context) error) error {
	if run != nil {
		return run(fn)
	}
	return fn(context.Background())
}
