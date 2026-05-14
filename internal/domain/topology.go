package domain

import "time"

const (
	LogStatusProcessing = "processing"
	LogStatusParsed     = "parsed"
	LogStatusFailed     = "failed"
)

type Log struct {
	ID           int64     `json:"id"`
	FilePath     string    `json:"file_path"`
	Status       string    `json:"status"`
	NodesCount   int       `json:"nodes_count"`
	PortsCount   int       `json:"ports_count"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

type Node struct {
	ID         int64      `json:"id"`
	LogID      int64      `json:"log_id,omitempty"`
	ExternalID string     `json:"external_id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	CreatedAt  time.Time  `json:"created_at,omitempty"`
	Ports      []Port     `json:"ports,omitempty"`
	Info       []NodeInfo `json:"info,omitempty"`
}

type Port struct {
	ID     int64  `json:"id"`
	NodeID int64  `json:"node_id,omitempty"`
	Name   string `json:"name"`
	MAC    string `json:"mac,omitempty"`
	IP     string `json:"ip,omitempty"`
	Status string `json:"status"`
	Speed  string `json:"speed,omitempty"`
}

type NodeInfo struct {
	ID     int64  `json:"id"`
	NodeID int64  `json:"node_id,omitempty"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

type ParsedLog struct {
	Nodes []Node
}

func (p ParsedLog) Counts() (int, int) {
	ports := 0
	for _, node := range p.Nodes {
		ports += len(node.Ports)
	}
	return len(p.Nodes), ports
}

type Topology struct {
	LogID int64  `json:"log_id"`
	Nodes []Node `json:"nodes"`
}
