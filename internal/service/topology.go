package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"topology-parser/internal/domain"
	"topology-parser/internal/parser"
	"topology-parser/internal/repository"
)

var (
	ErrInvalidPath = errors.New("invalid path")
	ErrNotFound    = repository.ErrNotFound
)

type TopologyRepository interface {
	CreateLog(ctx context.Context, filePath string) (int64, error)
	MarkLogFailed(ctx context.Context, logID int64, message string) error
	SaveParsedLog(ctx context.Context, logID int64, parsed domain.ParsedLog) error
	GetLog(ctx context.Context, logID int64) (domain.Log, error)
	GetTopology(ctx context.Context, logID int64) (domain.Topology, error)
	GetNode(ctx context.Context, nodeID int64) (domain.Node, error)
	GetPortsByNode(ctx context.Context, nodeID int64) ([]domain.Port, error)
}

type LogParser interface {
	ParseFile(ctx context.Context, path string) (domain.ParsedLog, error)
}

type TopologyService struct {
	repo    TopologyRepository
	parser  LogParser
	dataDir string
	logger  *slog.Logger
}

func NewTopologyService(repo TopologyRepository, parser LogParser, dataDir string, logger *slog.Logger) *TopologyService {
	return &TopologyService{
		repo:    repo,
		parser:  parser,
		dataDir: dataDir,
		logger:  logger,
	}
}

func NewDefaultTopologyService(repo TopologyRepository, dataDir string, logger *slog.Logger) *TopologyService {
	return NewTopologyService(repo, parser.New(), dataDir, logger)
}

type ParseResult struct {
	LogID  int64  `json:"log_id"`
	Status string `json:"status"`
}

func (s *TopologyService) Parse(ctx context.Context, requestedPath string) (ParseResult, error) {
	cleanPath, absPath, err := s.validatePath(requestedPath)
	if err != nil {
		return ParseResult{}, err
	}

	logID, err := s.repo.CreateLog(ctx, cleanPath)
	if err != nil {
		return ParseResult{}, err
	}

	parsed, err := s.parser.ParseFile(ctx, absPath)
	if err != nil {
		if markErr := s.repo.MarkLogFailed(ctx, logID, err.Error()); markErr != nil {
			s.logger.ErrorContext(ctx, "failed to mark log as failed", "log_id", logID, "error", markErr)
		}
		return ParseResult{LogID: logID, Status: domain.LogStatusFailed}, fmt.Errorf("parse log: %w", err)
	}

	if err := s.repo.SaveParsedLog(ctx, logID, parsed); err != nil {
		if markErr := s.repo.MarkLogFailed(ctx, logID, err.Error()); markErr != nil {
			s.logger.ErrorContext(ctx, "failed to mark log as failed", "log_id", logID, "error", markErr)
		}
		return ParseResult{LogID: logID, Status: domain.LogStatusFailed}, err
	}

	return ParseResult{LogID: logID, Status: domain.LogStatusParsed}, nil
}

func (s *TopologyService) GetTopology(ctx context.Context, logID int64) (domain.Topology, error) {
	return s.repo.GetTopology(ctx, logID)
}

func (s *TopologyService) GetNode(ctx context.Context, nodeID int64) (domain.Node, error) {
	return s.repo.GetNode(ctx, nodeID)
}

func (s *TopologyService) GetPortsByNode(ctx context.Context, nodeID int64) ([]domain.Port, error) {
	return s.repo.GetPortsByNode(ctx, nodeID)
}

func (s *TopologyService) GetLog(ctx context.Context, logID int64) (domain.Log, error) {
	return s.repo.GetLog(ctx, logID)
}

func (s *TopologyService) validatePath(requestedPath string) (string, string, error) {
	if requestedPath == "" || filepath.IsAbs(requestedPath) {
		return "", "", ErrInvalidPath
	}

	cleanPath := filepath.Clean(requestedPath)
	cleanDataDir := filepath.Clean(s.dataDir)
	if cleanPath == "." || cleanPath == cleanDataDir {
		return "", "", ErrInvalidPath
	}

	dataPrefix := cleanDataDir + string(filepath.Separator)
	if cleanPath != cleanDataDir && !strings.HasPrefix(cleanPath, dataPrefix) {
		return "", "", ErrInvalidPath
	}

	ext := strings.ToLower(filepath.Ext(cleanPath))
	if !allowedExtension(ext) {
		return "", "", ErrInvalidPath
	}

	absDataDir, err := filepath.Abs(cleanDataDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve data dir: %w", err)
	}
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve file path: %w", err)
	}

	rel, err := filepath.Rel(absDataDir, absPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", "", ErrInvalidPath
	}

	if info, err := filepath.EvalSymlinks(absPath); err == nil {
		resolvedRel, relErr := filepath.Rel(absDataDir, info)
		if relErr != nil || resolvedRel == "." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) || resolvedRel == ".." || filepath.IsAbs(resolvedRel) {
			return "", "", ErrInvalidPath
		}
	} else {
		return "", "", ErrInvalidPath
	}

	return cleanPath, absPath, nil
}

func allowedExtension(ext string) bool {
	switch ext {
	case ".log", ".txt", ".csv", ".db_csv", ".sharp_an_info":
		return true
	default:
		return false
	}
}
