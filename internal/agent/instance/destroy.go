package instance

import (
	"context"
	"log/slog"

	"github.com/valyentdev/ravel/pkg/core"
)

func (m *Manager) Destroy(ctx context.Context, force bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.state.Status() == core.MachineStatusDestroyed {
		return nil
	}
	if m.isRunning && !force {
		return core.NewFailedPrecondition("instance is running")
	}

	if m.isRunning {
		err := m.runtime.StopVM(context.Background(), m.state.Id(), "", 0)
		if err != nil {
			return err
		}

		<-m.waitCh
	}

	err := m.destroyImpl(ctx, core.OriginUser, force, "requested by user")
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) destroyImpl(ctx context.Context, origin core.Origin, force bool, reason string) error {
	err := m.state.PushInstanceDestroyEvent(ctx, origin, force, reason)
	if err != nil {
		return err
	}

	err = m.runtime.DestroyInstance(ctx, m.state.Instance().Id)
	if err != nil {
		slog.Error("failed to destroy instance", "error", err)
	}

	err = m.state.PushInstanceDestroyedEvent(ctx)
	if err != nil {
		return err
	}

	return nil
}
