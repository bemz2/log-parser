package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"log-parser/internal/domain"
)

type TopologyRepository struct {
	db *sql.DB
}

func NewTopologyRepository(db *sql.DB) *TopologyRepository {
	return &TopologyRepository{db: db}
}

func (r *TopologyRepository) CreateLog(ctx context.Context, filePath string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO logs(file_path, status) VALUES ($1, $2) RETURNING id`,
		filePath,
		domain.LogStatusProcessing,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create log: %w", err)
	}
	return id, nil
}

func (r *TopologyRepository) MarkLogFailed(ctx context.Context, logID int64, message string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE logs SET status = $1, error_message = $2 WHERE id = $3`,
		domain.LogStatusFailed,
		message,
		logID,
	)
	if err != nil {
		return fmt.Errorf("mark log failed: %w", err)
	}
	return nil
}

func (r *TopologyRepository) ProcessParsedLog(ctx context.Context, logID int64, parse func(context.Context) (domain.ParsedLog, error)) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	parsed, err := parse(ctx)
	if err != nil {
		return err
	}

	nodeStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO nodes(log_id, external_id, name, type)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`)
	if err != nil {
		return fmt.Errorf("prepare insert node: %w", err)
	}
	defer func() { _ = nodeStmt.Close() }()

	portStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ports(node_id, name, mac, ip, status, speed)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULLIF($6, ''))
	`)
	if err != nil {
		return fmt.Errorf("prepare insert port: %w", err)
	}
	defer func() { _ = portStmt.Close() }()

	infoStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO nodes_info(node_id, key, value)
		VALUES ($1, $2, $3)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert node info: %w", err)
	}
	defer func() { _ = infoStmt.Close() }()

	for _, node := range parsed.Nodes {
		var nodeID int64
		if err := nodeStmt.QueryRowContext(ctx, logID, node.ExternalID, node.Name, node.Type).Scan(&nodeID); err != nil {
			return fmt.Errorf("insert node %s: %w", node.ExternalID, err)
		}

		for _, port := range node.Ports {
			if _, err := portStmt.ExecContext(ctx, nodeID, port.Name, port.MAC, port.IP, port.Status, port.Speed); err != nil {
				return fmt.Errorf("insert port %s: %w", port.Name, err)
			}
		}

		for _, info := range node.Info {
			if _, err := infoStmt.ExecContext(ctx, nodeID, info.Key, info.Value); err != nil {
				return fmt.Errorf("insert node info %s: %w", info.Key, err)
			}
		}
	}

	nodesCount, portsCount := parsed.Counts()
	if _, err := tx.ExecContext(ctx, `
		UPDATE logs
		SET status = $1, nodes_count = $2, ports_count = $3, error_message = NULL
		WHERE id = $4
	`, domain.LogStatusParsed, nodesCount, portsCount, logID); err != nil {
		return fmt.Errorf("update parsed log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *TopologyRepository) GetLog(ctx context.Context, logID int64) (domain.Log, error) {
	var log domain.Log
	err := r.db.QueryRowContext(ctx, `
		SELECT id, file_path, status, nodes_count, ports_count, error_message, uploaded_at
		FROM logs
		WHERE id = $1
	`, logID).Scan(&log.ID, &log.FilePath, &log.Status, &log.NodesCount, &log.PortsCount, &log.ErrorMessage, &log.UploadedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Log{}, ErrNotFound
	}
	if err != nil {
		return domain.Log{}, fmt.Errorf("get log: %w", err)
	}
	return log, nil
}

func (r *TopologyRepository) GetTopology(ctx context.Context, logID int64) (domain.Topology, error) {
	if _, err := r.GetLog(ctx, logID); err != nil {
		return domain.Topology{}, err
	}

	nodes, err := r.listNodesByLogID(ctx, logID)
	if err != nil {
		return domain.Topology{}, err
	}

	for i := range nodes {
		ports, err := r.GetPortsByNode(ctx, nodes[i].ID)
		if err != nil {
			return domain.Topology{}, err
		}
		nodes[i].Ports = ports
	}

	return domain.Topology{LogID: logID, Nodes: nodes}, nil
}

func (r *TopologyRepository) GetNode(ctx context.Context, nodeID int64) (domain.Node, error) {
	nodes, err := r.listNodesByID(ctx, nodeID)
	if err != nil {
		return domain.Node{}, err
	}
	if len(nodes) == 0 {
		return domain.Node{}, ErrNotFound
	}

	node := nodes[0]
	ports, err := r.GetPortsByNode(ctx, nodeID)
	if err != nil {
		return domain.Node{}, err
	}
	info, err := r.getNodeInfo(ctx, nodeID)
	if err != nil {
		return domain.Node{}, err
	}
	node.Ports = ports
	node.Info = info
	return node, nil
}

func (r *TopologyRepository) GetPortsByNode(ctx context.Context, nodeID int64) ([]domain.Port, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, node_id, name, COALESCE(mac, ''), COALESCE(ip, ''), status, COALESCE(speed, '')
		FROM ports
		WHERE node_id = $1
		ORDER BY id
	`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query ports: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ports := make([]domain.Port, 0)
	for rows.Next() {
		var port domain.Port
		if err := rows.Scan(&port.ID, &port.NodeID, &port.Name, &port.MAC, &port.IP, &port.Status, &port.Speed); err != nil {
			return nil, fmt.Errorf("scan port: %w", err)
		}
		ports = append(ports, port)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ports: %w", err)
	}

	return ports, nil
}

func (r *TopologyRepository) listNodesByLogID(ctx context.Context, logID int64) ([]domain.Node, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, log_id, external_id, name, type, created_at
		FROM nodes
		WHERE log_id = $1
		ORDER BY id
	`, logID)
	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanNodes(rows)
}

func (r *TopologyRepository) listNodesByID(ctx context.Context, nodeID int64) ([]domain.Node, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, log_id, external_id, name, type, created_at
		FROM nodes
		WHERE id = $1
	`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanNodes(rows)
}

func scanNodes(rows *sql.Rows) ([]domain.Node, error) {
	nodes := make([]domain.Node, 0)
	for rows.Next() {
		var node domain.Node
		if err := rows.Scan(&node.ID, &node.LogID, &node.ExternalID, &node.Name, &node.Type, &node.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}

	return nodes, nil
}

func (r *TopologyRepository) getNodeInfo(ctx context.Context, nodeID int64) ([]domain.NodeInfo, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, node_id, key, value
		FROM nodes_info
		WHERE node_id = $1
		ORDER BY id
	`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query node info: %w", err)
	}
	defer func() { _ = rows.Close() }()

	info := make([]domain.NodeInfo, 0)
	for rows.Next() {
		var item domain.NodeInfo
		if err := rows.Scan(&item.ID, &item.NodeID, &item.Key, &item.Value); err != nil {
			return nil, fmt.Errorf("scan node info: %w", err)
		}
		info = append(info, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node info: %w", err)
	}

	return info, nil
}
