package service_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log-parser/internal/domain"
	"log-parser/internal/service"
	"log-parser/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const nodeTypeSwitch = "switch"

func TestTopologyServiceParseSuccess(t *testing.T) {
	ctx := context.Background()
	dataDir, requestedPath, absPath := createTestLog(t, "valid.log")
	parsed := domain.ParsedLog{
		Nodes: []domain.Node{
			{
				ExternalID: "switch-1",
				Name:       "switch-1",
				Type:       nodeTypeSwitch,
				Ports:      []domain.Port{{Name: "eth0", Status: "active"}},
			},
		},
	}

	repo := mocks.NewTopologyRepository(t)
	parser := mocks.NewLogParser(t)
	svc := service.NewTopologyService(repo, parser, dataDir, testLogger())

	repo.EXPECT().
		CreateLog(ctx, requestedPath).
		Return(int64(10), nil).
		Once()
	parser.EXPECT().
		ParseFile(ctx, absPath).
		Return(parsed, nil).
		Once()
	repo.EXPECT().
		ProcessParsedLog(ctx, int64(10), mock.Anything).
		RunAndReturn(func(txCtx context.Context, _ int64, parse func(context.Context) (domain.ParsedLog, error)) error {
			got, err := parse(txCtx)
			require.NoError(t, err)
			require.Equal(t, parsed, got)
			return nil
		}).
		Once()

	result, err := svc.Parse(ctx, requestedPath)

	require.NoError(t, err)
	require.Equal(t, service.ParseResult{LogID: 10, Status: domain.LogStatusParsed}, result)
}

func TestTopologyServiceParseRejectsInvalidPath(t *testing.T) {
	t.Parallel()

	repo := mocks.NewTopologyRepository(t)
	parser := mocks.NewLogParser(t)
	svc := service.NewTopologyService(repo, parser, "data", testLogger())

	result, err := svc.Parse(context.Background(), "../secret.log")

	require.ErrorIs(t, err, service.ErrInvalidPath)
	require.Empty(t, result)
	repo.AssertNotCalled(t, "CreateLog", mock.Anything, mock.Anything)
	parser.AssertNotCalled(t, "ParseFile", mock.Anything, mock.Anything)
}

func TestTopologyServiceParseMarksLogFailed(t *testing.T) {
	ctx := context.Background()
	dataDir, requestedPath, absPath := createTestLog(t, "broken.log")
	parseErr := errors.New("broken format")

	repo := mocks.NewTopologyRepository(t)
	parser := mocks.NewLogParser(t)
	svc := service.NewTopologyService(repo, parser, dataDir, testLogger())

	repo.EXPECT().
		CreateLog(ctx, requestedPath).
		Return(int64(11), nil).
		Once()
	parser.EXPECT().
		ParseFile(ctx, absPath).
		Return(domain.ParsedLog{}, parseErr).
		Once()
	repo.EXPECT().
		ProcessParsedLog(ctx, int64(11), mock.Anything).
		RunAndReturn(func(txCtx context.Context, _ int64, parse func(context.Context) (domain.ParsedLog, error)) error {
			_, err := parse(txCtx)
			return err
		}).
		Once()
	repo.EXPECT().
		MarkLogFailed(ctx, int64(11), parseErr.Error()).
		Return(nil).
		Once()

	result, err := svc.Parse(ctx, requestedPath)

	require.Error(t, err)
	require.Equal(t, service.ParseResult{LogID: 11, Status: domain.LogStatusFailed}, result)
}

func TestTopologyServiceReadMethodsDelegateToRepository(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := mocks.NewTopologyRepository(t)
	parser := mocks.NewLogParser(t)
	svc := service.NewTopologyService(repo, parser, "data", testLogger())

	expectedTopology := domain.Topology{LogID: 1, Nodes: []domain.Node{{ID: 2, Name: nodeTypeSwitch}}}
	expectedNode := domain.Node{ID: 2, Name: nodeTypeSwitch}
	expectedPorts := []domain.Port{{ID: 3, NodeID: 2, Name: "eth0"}}
	expectedLog := domain.Log{ID: 1, Status: domain.LogStatusParsed}

	repo.EXPECT().GetTopology(ctx, int64(1)).Return(expectedTopology, nil).Once()
	repo.EXPECT().GetNode(ctx, int64(2)).Return(expectedNode, nil).Once()
	repo.EXPECT().GetPortsByNode(ctx, int64(2)).Return(expectedPorts, nil).Once()
	repo.EXPECT().GetLog(ctx, int64(1)).Return(expectedLog, nil).Once()

	topology, err := svc.GetTopology(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, expectedTopology, topology)

	node, err := svc.GetNode(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, expectedNode, node)

	ports, err := svc.GetPortsByNode(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, expectedPorts, ports)

	log, err := svc.GetLog(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, expectedLog, log)
}

func createTestLog(t *testing.T, filename string) (string, string, string) {
	t.Helper()

	dataDir := filepath.Join("testdata", strings.ReplaceAll(t.Name(), "/", "_"))
	require.NoError(t, os.MkdirAll(dataDir, 0o755))
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dataDir)) })

	requestedPath := filepath.Join(dataDir, filename)
	require.NoError(t, os.WriteFile(requestedPath, []byte("START_NODES\n"), 0o600))

	absPath, err := filepath.Abs(requestedPath)
	require.NoError(t, err)

	return dataDir, requestedPath, absPath
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}
