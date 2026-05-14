package application

import (
	"context"
	"errors"
)

type App struct {
	container *Container
}

func NewApp(container *Container) *App {
	return &App{container: container}
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.container.PublicServer.Start()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		return nil
	}

	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	var result error
	if a.container.PublicServer != nil {
		result = errors.Join(result, a.container.PublicServer.Shutdown(ctx))
	}
	result = errors.Join(result, a.container.Close())
	return result
}
