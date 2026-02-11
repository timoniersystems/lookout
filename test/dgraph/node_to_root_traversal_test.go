package dgraph_test

import (
	"defender/pkg/gui/dgraph"
	"reflect"
	"testing"
)

func TestNodeToRootTraversal_SimpleChain(t *testing.T) {
	t.Log("Starting TestNodeToRootTraversal_SimpleChain")
	graph := map[string]dgraph.Component{
		"uid-a": {Uid: "uid-a", DependsOn: []dgraph.DependsOn{{Uid: "uid-b"}}},
		"uid-b": {Uid: "uid-b", DependsOn: []dgraph.DependsOn{{Uid: "uid-c"}}},
		"uid-c": {Uid: "uid-c", Root: true},
	}

	path := dgraph.FindShortestPathWithDepth(graph, "uid-a", "uid-c")
	t.Logf("Computed path: %v", path)
	expected := []string{"uid-a", "uid-b", "uid-c"}

	if !reflect.DeepEqual(path, expected) {
		t.Errorf("Expected path %v, got %v", expected, path)
	}
	t.Log("Finished TestNodeToRootTraversal_SimpleChain")
}

func TestNodeToRootTraversal_NoPath(t *testing.T) {
	t.Log("Starting TestNodeToRootTraversal_NoPath")
	graph := map[string]dgraph.Component{
		"uid-a": {Uid: "uid-a"},
		"uid-c": {Uid: "uid-c", Root: true},
	}

	path := dgraph.FindShortestPathWithDepth(graph, "uid-a", "uid-c")
	t.Logf("Computed path: %v", path)
	if len(path) != 0 {
		t.Errorf("Expected no path, got %v", path)
	}
	t.Log("Finished TestNodeToRootTraversal_NoPath")
}
