package instance

import (
	"context"
	"log/slog"
	"time"

	"github.com/valyentdev/ravel/internal/networking"
	"github.com/valyentdev/ravel/pkg/core"
	"github.com/valyentdev/ravel/pkg/runtimes"
)

const MAX_RETRIES = 3

func (m *Manager) Prepare() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.prepare()
}

func (m *Manager) prepare() error {
	slog.Info("preparing instance", "instance_id", m.state.Id())
	retries := 0
	lastEvent := m.state.LastEvent()
	if lastEvent.Type == core.InstancePrepare {
		retries = lastEvent.Payload.Prepare.Retries
	}

	ctx := context.Background()
	var err error
	var fatal bool

	for retries < MAX_RETRIES {
		err = m.state.PushInstancePrepareEvent(ctx, retries)
		if err != nil {
			return err
		}

		retries++

		err, fatal = m.runtime.PrepareInstance(ctx, m.state.Instance(), runtimes.NetworkConfig{
			LocalConfig: networking.LocalIPV4Subnet(m.reservation.LocalIPV4Subnet).LocalConfig(),
		})
		if err == nil {
			err = m.state.PushInstancePreparedEvent(ctx)
			if err != nil {
				return err
			}

			m.isPrepared = true
			if m.Instance().State.DesiredStatus == core.MachineStatusRunning {
				go m.Start(context.Background())
			}
			return nil
		}

		err = m.state.PushInstancePreparationFailedEvent(ctx, err.Error())
		if err != nil {
			slog.Error("failed to push instance preparation failed event", "error", err)
			return err
		}

		if fatal {
			break
		}

		slog.Warn("instance preparation failed. Retrying in 10 seconds", "error", err, "retries", retries)
		time.Sleep(10 * time.Second)
	}

	var reason string
	if fatal {
		reason = "instance preparation failed and is unrecoverable"
	} else {
		reason = "instance preparation failed after maximum retries"
	}

	destroyErr := m.destroyImpl(ctx, core.OriginRavel, false, reason)
	if destroyErr != nil {
		slog.Error("failed to destroy instance after preparation failed", "error", destroyErr)
		return err
	}

	return nil
}
