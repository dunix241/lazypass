package vault

import (
	"context"
	"errors"
	"fmt"

	"lazypass/internal/debug"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *View) cancelDelete() {
	s.deleteTarget, s.deleteDialog, s.deletePlan = nil, nil, nil
	s.deleteGeneration++
}

func (s *View) openDelete() {
	if s.deleting {
		return
	}

	node, ok := s.selectedVaultNode()
	if !ok || s.isVaultParentNode(node) || len(node.Path) == 0 {
		return
	}
	if node.Protected {
		s.notify(shared.Warning, "This location cannot be deleted")
		return
	}

	if s.vault.Provider == nil || !s.vault.Provider.Capabilities().Delete {
		s.notify(shared.Warning, "Deletion unavailable")
		return
	}

	node.Path = append(vault.Path(nil), node.Path...)
	s.deleteTarget = &node
	s.deletePlan = nil
	s.deleteGeneration++
	options := s.deleteConfirmationOptions(node)
	dialog := shared.NewConfirmation(options)
	s.deleteDialog = dialog

	if planner, ok := s.vault.Provider.(vault.PlannedDeleter); ok && node.Kind == vault.FolderNode && s.deleteRecursive {
		s.prepareDelete(planner, node, dialog, options.Message)
	}
}

func (s *View) deleteConfirmationOptions(node vault.Node) shared.ConfirmationOptions {
	options := shared.ConfirmationOptions{
		Title:       "Delete entry " + node.Name,
		Message:     "Deletes this entry from the vault",
		Destructive: true,
		OnConfirm:   s.performDelete,
		OnCancel:    s.cancelDelete,
	}

	if node.Shared {
		options.Title = "Delete shared entry " + node.Name
		options.Message = "Deletes this item from every collection"
	}
	if node.Kind == vault.FolderNode {
		options.Title = "Delete folder " + node.Name
		options.Message = "Moves all contents, including subfolders, to Unorganized. No entries will be deleted."
		if s.deleteRecursive {
			options.Title = "Delete folder recursively " + node.Name
			options.Message = "Deletes this folder and all entries beneath it"
		} else if s.vault.Provider.Name() == "bitwarden" {
			options.Message = "Removes this collection only. Entries and nested collections are preserved."
		}
		options.RequireText = node.Name
		options.ConfirmLabel = "delete"
	}

	return options
}

func (s *View) prepareDelete(planner vault.PlannedDeleter, node vault.Node, dialog *shared.Confirmation, fallbackMessage string) {
	generation := s.deleteGeneration
	s.deleting, dialog.Busy = true, true
	dialog.Options.Message = "Preparing deletion…"

	var plan vault.DeletePlan
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		var err error
		plan, err = planner.PlanDelete(ctx, node.Path)
		return err
	}, func(err error) {
		s.deleting = false
		if generation != s.deleteGeneration || s.deleteDialog != dialog {
			return
		}

		dialog.Busy = false
		if errors.Is(err, vault.ErrUnsupported) && (len(node.Path) == 0 || node.Path[0] != "Organizations") {
			dialog.Options.Message = fallbackMessage
			return
		}
		if err != nil {
			s.cancelDelete()
			s.notify(shared.Error, storeErrorMessage(err))
			return
		}

		s.deletePlan = &plan
		dialog.Options.Message = fmt.Sprintf("Deletes %d collections and %d entries. Shared entries are deleted from every collection.", plan.Containers, plan.Entries)
	})
}

func (s *View) handleDeleteKey(event *tcell.EventKey) {
	if s.deleteDialog != nil {
		s.deleteDialog.HandleKey(event)
	}
}

func (s *View) PasteConfirmation(text string) {
	if s.deleteDialog != nil {
		s.deleteDialog.Paste(text)
	}
}

func (s *View) performDelete() {
	if s.deleteTarget == nil || s.deleting {
		return
	}

	node, plan, recursive := *s.deleteTarget, s.deletePlan, s.deleteRecursive
	s.cancelDelete()
	s.deleting = true
	s.progress("Deleting…")

	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		if plan != nil {
			return s.vault.Provider.(vault.PlannedDeleter).ExecuteDelete(ctx, *plan)
		}
		if node.Kind == vault.FolderNode && !recursive {
			return s.vault.RemoveFolder(ctx, node.Path)
		}
		return s.vault.Delete(ctx, node)
	}, func(err error) {
		s.deleting = false

		if err != nil {
			s.Refresh()
			debug.Failure("vault delete", vaultFailureCause(err))
			message := "Delete failed"
			if errors.Is(err, vault.ErrUnsupported) {
				message = "Deletion unavailable"
			}
			var diagnostic vault.Diagnostic
			if errors.As(err, &diagnostic) {
				message = diagnostic.UserMessage()
			}
			s.notify(shared.Error, message)
			return
		}
		s.Reconcile("Deleted from vault")
	})
}

func (s *View) drawDelete(screen tcell.Screen, frame shared.Frame) {
	if s.deleteDialog != nil {
		s.deleteDialog.Draw(screen, frame)
	}
}
