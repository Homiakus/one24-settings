package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type algorithmRequest struct {
	ID         string               `json:"id"`
	Nodes      []algorithmNode      `json:"nodes"`
	Edges      []algorithmEdge      `json:"edges"`
	Parameters []algorithmParameter `json:"parameters,omitempty"`
	Subgraphs  []algorithmSubgraph  `json:"subgraphs,omitempty"`
}

type algorithmNode struct {
	ID      string            `json:"id"`
	Kind    string            `json:"kind"`
	Label   string            `json:"label"`
	Variant string            `json:"variant,omitempty"`
	Command int               `json:"command,omitempty"`
	Config  map[string]string `json:"config,omitempty"`
}

type algorithmEdge struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
	Priority  int    `json:"priority,omitempty"`
}

type algorithmParameter struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Default  string `json:"default,omitempty"`
}

type algorithmSubgraph struct {
	ID      string               `json:"id"`
	Name    string               `json:"name"`
	Inputs  []algorithmParameter `json:"inputs,omitempty"`
	Entry   string               `json:"entry"`
	Exit    string               `json:"exit"`
	NodeIDs []string             `json:"nodeIds"`
}

func validateAlgorithm(request algorithmRequest) error {
	if request.ID == "" {
		return fmt.Errorf("algorithm id is required")
	}
	nodes := make(map[string]struct{}, len(request.Nodes))
	for _, node := range request.Nodes {
		if node.ID == "" || node.Kind == "" {
			return fmt.Errorf("algorithm nodes require id and kind")
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("duplicate algorithm node %q", node.ID)
		}
		nodes[node.ID] = struct{}{}
	}
	edges := make(map[string]struct{}, len(request.Edges))
	for _, edge := range request.Edges {
		if edge.ID == "" || edge.From == "" || edge.To == "" {
			return fmt.Errorf("algorithm edges require id, from and to")
		}
		if edge.From == edge.To {
			return fmt.Errorf("algorithm edge %q cannot connect node to itself", edge.ID)
		}
		if _, exists := edges[edge.ID]; exists {
			return fmt.Errorf("duplicate algorithm edge %q", edge.ID)
		}
		if _, exists := nodes[edge.From]; !exists {
			return fmt.Errorf("edge %q references missing source %q", edge.ID, edge.From)
		}
		if _, exists := nodes[edge.To]; !exists {
			return fmt.Errorf("edge %q references missing target %q", edge.ID, edge.To)
		}
		edges[edge.ID] = struct{}{}
	}
	for _, parameter := range request.Parameters {
		if parameter.Name == "" || parameter.Type == "" {
			return fmt.Errorf("algorithm parameters require name and type")
		}
	}
	for _, subgraph := range request.Subgraphs {
		if subgraph.ID == "" || subgraph.Name == "" || subgraph.Entry == "" || subgraph.Exit == "" {
			return fmt.Errorf("subgraphs require id, name, entry and exit")
		}
		if _, ok := nodes[subgraph.Entry]; !ok {
			return fmt.Errorf("subgraph %q entry %q is missing", subgraph.ID, subgraph.Entry)
		}
		if _, ok := nodes[subgraph.Exit]; !ok {
			return fmt.Errorf("subgraph %q exit %q is missing", subgraph.ID, subgraph.Exit)
		}
		for _, input := range subgraph.Inputs {
			if input.Name == "" || input.Type == "" {
				return fmt.Errorf("subgraph %q has invalid input parameter", subgraph.ID)
			}
		}
	}
	return nil
}

func (s *Server) handleAutoTraceAlgorithmValidate(w http.ResponseWriter, r *http.Request) {
	var request algorithmRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAutoTraceSceneBytes)).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "некорректная модель алгоритма: "+err.Error())
		return
	}
	if err := validateAlgorithm(request); err != nil {
		jsonError(w, http.StatusBadRequest, "алгоритм невалиден: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"valid": true, "nodes": len(request.Nodes), "edges": len(request.Edges), "subgraphs": len(request.Subgraphs)})
}
