package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/salsgithub/godst/graph"
	"github.com/stretchr/testify/require"
)

func TestGraphVizExporter_Export(t *testing.T) {
	t.Run("errors when creating directory fails", func(t *testing.T) {
		tempDirectory := t.TempDir()
		conflictingPath := filepath.Join(tempDirectory, "somefile")
		err := os.WriteFile(conflictingPath, []byte("hi"), 0644)
		require.NoError(t, err)
		gve := NewGraphVizExporter(conflictingPath)
		g := graph.New[string]()
		err = gve.Export(g)
		require.Error(t, err)
	})
	t.Run("errors when file write fails", func(t *testing.T) {
		tempDirectory := t.TempDir()
		conflictingPath := filepath.Join(tempDirectory, "graph.dot")
		err := os.MkdirAll(conflictingPath, 0755)
		require.NoError(t, err)
		gve := NewGraphVizExporter(tempDirectory)
		g := graph.New[string]()
		err = gve.Export(g)
		require.Error(t, err)
	})
	t.Run("handles an empty graph", func(t *testing.T) {
		tempDirectory := t.TempDir()
		gve := NewGraphVizExporter(tempDirectory)
		g := graph.New[string]()
		err := gve.Export(g)
		require.NoError(t, err)
		filePath := filepath.Join(tempDirectory, "graph.dot")
		b, err := os.ReadFile(filePath)
		require.NoError(t, err)
		want := `digraph G{
			rankdir="LR";
			node [shape=circle];
		}`
		wantLines := strings.Split(strings.TrimSpace(want), "\n")
		gotLines := strings.Split(strings.TrimSpace(string(b)), "\n")
		for i := range wantLines {
			wantLines[i] = strings.TrimSpace(wantLines[i])
		}
		for i := range gotLines {
			gotLines[i] = strings.TrimSpace(gotLines[i])
		}
		require.Equal(t, wantLines, gotLines)
	})
	t.Run("handles populated graph", func(t *testing.T) {
		tempDirectory := t.TempDir()
		gve := NewGraphVizExporter(tempDirectory)
		g := graph.New[string]()
		g.AddEdge("A", "B", 10)
		g.AddNode("C")
		err := gve.Export(g)
		require.NoError(t, err)
		filePath := filepath.Join(tempDirectory, "graph.dot")
		b, err := os.ReadFile(filePath)
		require.NoError(t, err)
		want := `digraph G{
			rankdir="LR";
			node [shape=circle];
			"A";
			"A" -> "B" [label="10"];
			"B";
			"C";
		}`
		wantLines := strings.Split(strings.TrimSpace(want), "\n")
		gotLines := strings.Split(strings.TrimSpace(string(b)), "\n")
		for i := range wantLines {
			wantLines[i] = strings.TrimSpace(wantLines[i])
		}
		for i := range gotLines {
			gotLines[i] = strings.TrimSpace(gotLines[i])
		}
		require.Equal(t, wantLines, gotLines)
	})
}
