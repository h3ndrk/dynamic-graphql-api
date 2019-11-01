package graph

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

type Graph struct {
	Nodes []*Node
	Edges []*Edge
}

func (g Graph) String() string {
	var stringNodes []string
	for _, n := range g.Nodes {
		stringNodes = append(stringNodes, fmt.Sprintf("Node %p: %+v", n, n.Attrs))
	}
	var stringEdges []string
	for _, e := range g.Edges {
		stringEdges = append(stringEdges, fmt.Sprintf("Edge %p -> %p: %+v", e.From, e.To, e.Attrs))
	}

	return strings.Join(append(stringNodes, stringEdges...), "\n")
}

func (g *Graph) AddNode(attrs map[string]string) *Node {
	n := &Node{
		Attrs: attrs,
	}

	fmt.Printf("AddNode(%+v) -> %p\n", attrs, n)

	g.Nodes = append(g.Nodes, n)

	return n
}

func (g *Graph) AddEdge(from, to *Node, attrs map[string]string) *Edge {
	e := &Edge{
		From:  from,
		To:    to,
		Attrs: attrs,
	}

	fmt.Printf("AddEdge(%p, %p, %+v)\n", from, to, attrs)

	g.Edges = append(g.Edges, e)

	return e
}

func (g Graph) EdgesByFrom(from *Node) []*Edge {
	var edges []*Edge
	for _, e := range g.Edges {
		if e.From == from {
			edges = append(edges, e)
		}
	}

	return edges
}

func (g Graph) EdgesByFromWithFilter(from *Node, f func(e *Edge) bool) []*Edge {
	var edges []*Edge
	for _, e := range g.Edges {
		if e.From == from && f(e) {
			edges = append(edges, e)
		}
	}

	return edges
}

func (g Graph) NodeByFilter(f func(n *Node) bool) (*Node, error) {
	for _, n := range g.Nodes {
		if f(n) {
			return n, nil
		}
	}

	return nil, errors.New("no node matched by filter")
}

func (g Graph) NodesByFilter(f func(n *Node) bool) []*Node {
	var nodes []*Node
	for _, n := range g.Nodes {
		if f(n) {
			nodes = append(nodes, n)
		}
	}

	return nodes
}

type Node struct {
	Attrs map[string]string
}

func (n Node) HasAttrKey(attrKey string) bool {
	_, ok := n.Attrs[attrKey]
	return ok
}

func (n Node) HasAttrValue(attrKey string, attrValue string) bool {
	if v, ok := n.Attrs[attrKey]; ok {
		return v == attrValue
	}

	return false
}

func (n Node) GetAttrValueDefault(attrKey string, defaultValue string) string {
	if v, ok := n.Attrs[attrKey]; ok {
		return v
	}

	return defaultValue
}

type Edge struct {
	From  *Node
	To    *Node
	Attrs map[string]string
}

func (e Edge) HasAttrKey(attrKey string) bool {
	_, ok := e.Attrs[attrKey]
	return ok
}

func (e Edge) HasAttrValue(attrKey string, attrValue string) bool {
	if v, ok := e.Attrs[attrKey]; ok {
		return v == attrValue
	}

	return false
}

func (e Edge) GetAttrValueDefault(attrKey string, defaultValue string) string {
	if v, ok := e.Attrs[attrKey]; ok {
		return v
	}

	return defaultValue
}
