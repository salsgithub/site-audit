package exporter

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/salsgithub/godst/graph"
)

type GraphVizExporter struct {
	path string
}

func NewGraphVizExporter(path string) *GraphVizExporter {
	return &GraphVizExporter{path: path}
}

func (g *GraphVizExporter) Export(gr *graph.Graph[string]) error {
	builder := strings.Builder{}
	builder.WriteString("digraph G{\n")
	builder.WriteString("  rankdir=\"LR\";\n")
	builder.WriteString("  node [shape=circle];\n")
	for _, node := range gr.Nodes() {
		builder.WriteString(fmt.Sprintf("  \"%v\";\n", node))
		neighbours, _ := gr.Neighbours(node)
		for _, neighbour := range neighbours {
			builder.WriteString(fmt.Sprintf("  \"%v\" -> \"%v\" [label=\"%d\"];\n", node, neighbour.Link, neighbour.Weight))
		}
	}
	builder.WriteString("}\n")
	if err := os.MkdirAll(g.path, 0755); err != nil {
		return err
	}
	contents := builder.String()
	return os.WriteFile(path.Join(g.path, "graph.dot"), []byte(contents), 0644)
}
