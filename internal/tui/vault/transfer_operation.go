package vault

import (
	"context"

	"lazypass/internal/vault"
)

// transferNode keeps folder capability checks consistent during preflight and
// execution, including providers which only support entry transfers.
func (s *View) transferNode(ctx context.Context, node vault.Node, destination vault.Path, name string, move, check bool) error {
	if node.Kind == vault.FolderNode {
		provider, ok := s.vault.Provider.(vault.FolderTransfer)
		if !ok {
			return vault.ErrUnsupported
		}
		if check {
			return provider.CheckFolderTransfer(ctx, node.Path, destination, name, move)
		}
		return provider.TransferFolder(ctx, node.Path, destination, name, move)
	}
	provider, ok := s.vault.Provider.(vault.EntryTransfer)
	if !ok {
		return vault.ErrUnsupported
	}
	if check {
		return provider.CheckTransfer(ctx, node.Path, destination, name, move)
	}
	return provider.Transfer(ctx, node.Path, destination, name, move)
}

// operationContext propagates explicit cancellation across prompted retries.
func operationContext(parent, request context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	if request.Err() != nil {
		cancel()
	}
	unregister := context.AfterFunc(request, cancel)
	return ctx, func() { unregister(); cancel() }
}
