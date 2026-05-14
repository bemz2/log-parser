package parser

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"log-parser/internal/domain"
)

type Parser struct{}

func New() *Parser {
	return &Parser{}
}

func (p *Parser) ParseFile(ctx context.Context, path string) (domain.ParsedLog, error) {
	file, err := os.Open(path)
	if err != nil {
		return domain.ParsedLog{}, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	return p.Parse(ctx, file)
}

func (p *Parser) Parse(ctx context.Context, reader io.Reader) (domain.ParsedLog, error) {
	state := parseState{
		nodes:      make(map[string]*domain.Node),
		nodeOrder:  make([]string, 0),
		infoNodeID: "",
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	for lineNo := 1; scanner.Scan(); lineNo++ {
		if err := ctx.Err(); err != nil {
			return domain.ParsedLog{}, err
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || isSeparator(line) {
			continue
		}

		if err := state.consume(lineNo, line); err != nil {
			return domain.ParsedLog{}, err
		}
	}

	if err := scanner.Err(); err != nil {
		return domain.ParsedLog{}, fmt.Errorf("scan file: %w", err)
	}

	result := state.result()
	if len(result.Nodes) == 0 {
		return domain.ParsedLog{}, errors.New("log does not contain nodes")
	}

	return result, nil
}

type section string

const (
	sectionNone    section = ""
	sectionNodes   section = "nodes"
	sectionPorts   section = "ports"
	sectionUnknown section = "unknown"
)

type parseState struct {
	section section
	header  []string

	nodes     map[string]*domain.Node
	nodeOrder []string

	infoNodeID string
	unknownEnd string
}

func (s *parseState) consume(lineNo int, line string) error {
	switch line {
	case "START_NODES":
		s.section = sectionNodes
		s.header = nil
		return nil
	case "END_NODES":
		if s.section != sectionNodes {
			return fmt.Errorf("line %d: END_NODES without START_NODES", lineNo)
		}
		s.section = sectionNone
		s.header = nil
		return nil
	case "START_PORTS":
		s.section = sectionPorts
		s.header = nil
		return nil
	case "END_PORTS":
		if s.section != sectionPorts {
			return fmt.Errorf("line %d: END_PORTS without START_PORTS", lineNo)
		}
		s.section = sectionNone
		s.header = nil
		return nil
	}

	if strings.HasPrefix(line, "START_") {
		s.section = sectionUnknown
		s.header = nil
		s.unknownEnd = "END_" + strings.TrimPrefix(line, "START_")
		return nil
	}
	if s.section == sectionUnknown {
		if line == s.unknownEnd {
			s.section = sectionNone
			s.unknownEnd = ""
		}
		return nil
	}

	if strings.HasPrefix(line, "SW_GUID=") {
		externalID := strings.TrimSpace(strings.TrimPrefix(line, "SW_GUID="))
		if externalID == "" {
			return fmt.Errorf("line %d: empty SW_GUID", lineNo)
		}
		s.infoNodeID = externalID
		s.ensureNode(externalID, externalID, "switch")
		return nil
	}

	if s.section == sectionNodes || s.section == sectionPorts {
		return s.consumeCSV(lineNo, line)
	}

	if strings.Contains(line, "=") && s.infoNodeID != "" {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("line %d: invalid key-value line", lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("line %d: empty info key", lineNo)
		}
		node := s.nodes[s.infoNodeID]
		node.Info = append(node.Info, domain.NodeInfo{Key: key, Value: value})
		return nil
	}

	return fmt.Errorf("line %d: unexpected line %q", lineNo, line)
}

func (s *parseState) consumeCSV(lineNo int, line string) error {
	fields, err := readCSVLine(line)
	if err != nil {
		return fmt.Errorf("line %d: parse csv: %w", lineNo, err)
	}

	if s.header == nil {
		s.header = fields
		return nil
	}

	if len(fields) != len(s.header) {
		return fmt.Errorf("line %d: got %d fields, want %d", lineNo, len(fields), len(s.header))
	}

	row := make(map[string]string, len(fields))
	for i, key := range s.header {
		row[key] = strings.TrimSpace(fields[i])
	}

	switch s.section {
	case sectionNodes:
		return s.consumeNodeRow(lineNo, row)
	case sectionPorts:
		return s.consumePortRow(lineNo, row)
	case sectionNone, sectionUnknown:
		return fmt.Errorf("line %d: csv row outside section", lineNo)
	default:
		return fmt.Errorf("line %d: csv row outside section", lineNo)
	}
}

func (s *parseState) consumeNodeRow(lineNo int, row map[string]string) error {
	externalID := row["NodeGUID"]
	name := row["NodeDesc"]
	nodeType := mapNodeType(row["NodeType"])
	if externalID == "" || name == "" {
		return fmt.Errorf("line %d: node row must contain NodeGUID and NodeDesc", lineNo)
	}

	node := s.ensureNode(externalID, name, nodeType)
	appendInfo(node, "num_ports", row["NumPorts"])
	appendInfo(node, "class_version", row["ClassVersion"])
	appendInfo(node, "base_version", row["BaseVersion"])
	appendInfo(node, "system_image_guid", row["SystemImageGUID"])
	appendInfo(node, "port_guid", row["PortGUID"])
	return nil
}

func (s *parseState) consumePortRow(lineNo int, row map[string]string) error {
	externalID := row["NodeGuid"]
	if externalID == "" {
		return fmt.Errorf("line %d: port row must contain NodeGuid", lineNo)
	}

	node := s.nodes[externalID]
	if node == nil {
		return fmt.Errorf("line %d: port references unknown node %s", lineNo, externalID)
	}

	portNum := row["PortNum"]
	if portNum == "" {
		return fmt.Errorf("line %d: port row must contain PortNum", lineNo)
	}

	node.Ports = append(node.Ports, domain.Port{
		Name:   "port-" + portNum,
		MAC:    row["PortGuid"],
		IP:     normalizeIP(row["LID"]),
		Status: mapPortStatus(row["PortState"]),
		Speed:  row["LinkSpeedActv"],
	})
	return nil
}

func (s *parseState) ensureNode(externalID, name, nodeType string) *domain.Node {
	if node := s.nodes[externalID]; node != nil {
		if node.Name == "" {
			node.Name = name
		}
		if node.Type == "" {
			node.Type = nodeType
		}
		return node
	}

	node := &domain.Node{
		ExternalID: externalID,
		Name:       name,
		Type:       nodeType,
		Ports:      make([]domain.Port, 0),
		Info:       make([]domain.NodeInfo, 0),
	}
	s.nodes[externalID] = node
	s.nodeOrder = append(s.nodeOrder, externalID)
	return node
}

func (s *parseState) result() domain.ParsedLog {
	nodes := make([]domain.Node, 0, len(s.nodeOrder))
	for _, externalID := range s.nodeOrder {
		nodes = append(nodes, *s.nodes[externalID])
	}
	return domain.ParsedLog{Nodes: nodes}
}

func readCSVLine(line string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(line))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	return reader.Read()
}

func isSeparator(line string) bool {
	return strings.Trim(line, "-") == ""
}

func mapNodeType(value string) string {
	switch value {
	case "1":
		return "host"
	case "2":
		return "switch"
	default:
		if value == "" {
			return "unknown"
		}
		return value
	}
}

func mapPortStatus(value string) string {
	switch value {
	case "4":
		return "active"
	case "2":
		return "init"
	case "1":
		return "down"
	case "0":
		return "disabled"
	default:
		if value == "" || strings.EqualFold(value, "N/A") {
			return "unknown"
		}
		return value
	}
}

func normalizeIP(value string) string {
	if value == "" || value == "0" || strings.EqualFold(value, "N/A") {
		return ""
	}
	if _, err := strconv.Atoi(value); err != nil {
		return ""
	}
	return value
}

func appendInfo(node *domain.Node, key, value string) {
	if value == "" {
		return
	}
	node.Info = append(node.Info, domain.NodeInfo{Key: key, Value: value})
}
