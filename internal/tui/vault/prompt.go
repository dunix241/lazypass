package vault

import "context"

type PromptRunner func(func(context.Context) error) error

func (run PromptRunner) Execute(fn func(context.Context) error) error {
	if run != nil {
		return run(fn)
	}
	return fn(context.Background())
}
