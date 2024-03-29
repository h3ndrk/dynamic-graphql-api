package graph

import (
	"fmt"
	"strings"
)

// Graph consists of nodes and edges.
type Graph struct {
	nodes []*Node
	edges []*Edge
}

// NewGraph creates a new graph based on SQL statements.
func NewGraph(sqls []string) (*Graph, error) {
	g := &Graph{}
	if err := g.AddStmts(sqls); err != nil {
		return nil, err
	}
	if err := g.AddForeignKeyReferences(); err != nil {
		return nil, err
	}
	if err := g.MarkJoinTables(); err != nil {
		return nil, err
	}
	if err := g.AddObjects(); err != nil {
		return nil, err
	}

	return g, nil
}

// Nodes represents a subset of nodes of a graph.
type Nodes struct {
	graph *Graph
	nodes []*Node
}

// Nodes returns all nodes from a graph.
func (g *Graph) Nodes() Nodes {
	return Nodes{graph: g, nodes: g.nodes}
}

// Filter nodes with a function.
func (n Nodes) Filter(f func(n *Node) bool) Nodes {
	nodes := Nodes{graph: n.graph}
	for _, n := range n.nodes {
		if f(n) {
			nodes.nodes = append(nodes.nodes, n)
		}
	}

	return nodes
}

// Len calculates the amount of nodes.
func (n Nodes) Len() int {
	return len(n.nodes)
}

// First returns the first node. If the subset is empty nil is returned.
func (n Nodes) First() *Node {
	if len(n.nodes) < 1 {
		return nil
	}

	return n.nodes[0]
}

// All returns all nodes.
func (n Nodes) All() []*Node {
	return n.nodes
}

// ForEach executes a function on all nodes. If the function returns true the loop will continue, if false the loop with stop.
func (n Nodes) ForEach(f func(n *Node) bool) Nodes {
	for _, n := range n.nodes {
		if !f(n) {
			break
		}
	}

	return n
}

// Edges represents a subset of edges of a graph.
type Edges struct {
	graph *Graph
	edges []*Edge
}

// Edges returns all edges from a graph.
func (g *Graph) Edges() Edges {
	return Edges{graph: g, edges: g.edges}
}

// Filter edges with a function.
func (e Edges) Filter(f func(e *Edge) bool) Edges {
	edges := Edges{graph: e.graph}
	for _, e := range e.edges {
		if f(e) {
			edges.edges = append(edges.edges, e)
		}
	}

	return edges
}

// FilterSource filters edges to all edges that have the given source node.
func (e Edges) FilterSource(n *Node) Edges {
	edges := Edges{graph: e.graph}
	for _, e := range e.edges {
		if e.From == n {
			edges.edges = append(edges.edges, e)
		}
	}

	return edges
}

// FilterTarget filters edges to all edges that have the given target node.
func (e Edges) FilterTarget(n *Node) Edges {
	edges := Edges{graph: e.graph}
	for _, e := range e.edges {
		if e.To == n {
			edges.edges = append(edges.edges, e)
		}
	}

	return edges
}

// Sources returns from all edges the source nodes.
func (e Edges) Sources() Nodes {
	nodes := Nodes{graph: e.graph}
	for _, e := range e.edges {
		nodes.nodes = append(nodes.nodes, e.From)
	}

	return nodes
}

// Targets returns from all edges the target nodes.
func (e Edges) Targets() Nodes {
	nodes := Nodes{graph: e.graph}
	for _, e := range e.edges {
		nodes.nodes = append(nodes.nodes, e.To)
	}

	return nodes
}

// Len calculates the amount of edges.
func (e Edges) Len() int {
	return len(e.edges)
}

// First returns the first edge. If the subset is empty nil is returned.
func (e Edges) First() *Edge {
	if len(e.edges) < 1 {
		return nil
	}

	return e.edges[0]
}

// All returns all edges.
func (e Edges) All() []*Edge {
	return e.edges
}

// ForEach executes a function on all edges. If the function returns true the loop will continue, if false the loop with stop.
func (e Edges) ForEach(f func(e *Edge) bool) Edges {
	for _, e := range e.edges {
		if !f(e) {
			break
		}
	}

	return e
}

func (g Graph) String() string {
	var stringNodes []string
	for _, n := range g.nodes {
		stringNodes = append(stringNodes, fmt.Sprintf("Node %p: %+v", n, n.Attrs))
	}
	var stringEdges []string
	for _, e := range g.edges {
		stringEdges = append(stringEdges, fmt.Sprintf("Edge %p -> %p: %+v", e.From, e.To, e.Attrs))
	}

	return strings.Join(append(stringNodes, stringEdges...), "\n")
}

func (g *Graph) addNode(attrs map[string]string) *Node {
	n := &Node{
		Attrs: attrs,
	}

	fmt.Printf("AddNode(%+v) -> %p\n", attrs, n)

	g.nodes = append(g.nodes, n)

	return n
}

func (g *Graph) addEdge(from, to *Node, attrs map[string]string) *Edge {
	e := &Edge{
		From:  from,
		To:    to,
		Attrs: attrs,
	}

	fmt.Printf("AddEdge(%p, %p, %+v)\n", from, to, attrs)

	g.edges = append(g.edges, e)

	return e
}

// Node is a node in a graph which has attributes.
type Node struct {
	Attrs map[string]string
}

// HasAttrKey returns true if the node has the given attribute key.
func (n Node) HasAttrKey(attrKey string) bool {
	_, ok := n.Attrs[attrKey]
	return ok
}

// HasAttrValue returns true if the given attribute key of the node contains the given value.
func (n Node) HasAttrValue(attrKey string, attrValue string) bool {
	if v, ok := n.Attrs[attrKey]; ok {
		return v == attrValue
	}

	return false
}

// GetAttrValueDefault returns the value of a given attribute key or the given default if the attribute key does not exist.
func (n Node) GetAttrValueDefault(attrKey string, defaultValue string) string {
	if v, ok := n.Attrs[attrKey]; ok {
		return v
	}

	return defaultValue
}

// Edge is an edge in a graph which has attributes.
type Edge struct {
	From  *Node
	To    *Node
	Attrs map[string]string
}

// HasAttrKey returns true if the edge has the given attribute key.
func (e Edge) HasAttrKey(attrKey string) bool {
	_, ok := e.Attrs[attrKey]
	return ok
}

// HasAttrValue returns true if the given attribute key of the edge contains the given value.
func (e Edge) HasAttrValue(attrKey string, attrValue string) bool {
	if v, ok := e.Attrs[attrKey]; ok {
		return v == attrValue
	}

	return false
}

// GetAttrValueDefault returns the value of a given attribute key or the given default if the attribute key does not exist.
func (e Edge) GetAttrValueDefault(attrKey string, defaultValue string) string {
	if v, ok := e.Attrs[attrKey]; ok {
		return v
	}

	return defaultValue
}
