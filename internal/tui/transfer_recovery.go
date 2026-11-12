package tui

import (
	"context"
	"fmt"
	"lazypass/internal/task"
	"lazypass/internal/tui/notification"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"time"
)

func (s *screen) transferRecoveryTask() task.Task {
	return task.Task{ID: "transfer recovery", Delay: 10 * time.Second, Interval: 30 * time.Second, Run: func(ctx context.Context) error {
		s.dispatch(ctx, func() {
			if s.recoveryBusy || s.overlay != noOverlay || s.unlock != nil || s.vault.IsTransferActive() {
				return
			}
			active := s.vaults[s.vaultIndex].Service.Provider
			provider, ok := active.(vault.TransferRecoveryProvider)
			if !ok {
				return
			}
			s.recoveryBusy = true
			activation := s.vaultActivation
			s.operationRunner(active)(func(ctx context.Context) error {
				result, err := provider.RecoverTransfers(vault.WithoutPrompt(ctx))
				s.dispatch(ctx, func() {
					if activation != s.vaultActivation {
						return
					}
					if err != nil {
						status := "Transfer recovery is waiting for vault access; pending writes were retained"
						if status != s.recoveryStatus {
							s.recoveryStatus = status
							s.notifications.Show(notification.Message{ID: "transfer recovery", Text: status, Level: shared.Warning, Duration: -1}, time.Now())
						}
						return
					}
					status := fmt.Sprintf("Transfer recovery: %d completed · %d pending · %d failed", result.Completed, result.Pending, result.Failed)
					if result.Completed+result.Pending+result.Failed == 0 {
						s.recoveryStatus = ""
						s.notifications.Remove("transfer recovery", time.Now())
						return
					}
					if status != s.recoveryStatus {
						s.recoveryStatus = status
						level := shared.Info
						if result.Failed > 0 {
							level = shared.Warning
						}
						duration := time.Duration(-1)
						if result.Pending+result.Failed == 0 {
							level = shared.Success
							duration = 8 * time.Second
						}
						s.notifications.Show(notification.Message{ID: "transfer recovery", Text: status, Level: level, Duration: duration}, time.Now())
					}
					if result.Completed > 0 {
						s.vault.Refresh()
					}
				})
				// Errors are presented above, not returned to the prompted runner.
				return nil
			}, func(error) { s.recoveryBusy = false })
		})
		return nil
	}}
}
