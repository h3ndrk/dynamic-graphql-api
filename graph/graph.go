package graph

import "fmt"

type Graph struct {
	Nodes []*Node
	Edges []*Edge
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

// func (g *Graph) First(f func(n *Node) bool) *Node {
// 	for i := range g.Nodes {
// 		if f(&g.Nodes[i]) {
// 			return &g.Nodes[i]
// 		}
// 	}

// 	return nil
// }

type Node struct {
	Attrs map[string]string
}

type Edge struct {
	From  *Node
	To    *Node
	Attrs map[string]string
}
