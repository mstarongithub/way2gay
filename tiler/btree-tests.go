package tiler

import (
	"testing"

	generaldata "github.com/mstarongithub/way2gay/general-data"
)

func TestBTreeCreate(t *testing.T) {
	tree := NewTree(generaldata.Vector2i{X: 0, Y: 0})
	if tree.LastFocusedContainer == nil {
		t.Errorf("Last focused container is nil")
	}
	if tree.LastFocusedContainer != tree.Root.Leaf {
		t.Errorf("Last focused container is not the root leaf")
	}
	if tree.Root.Leaf.leafID != EMPTY_LEAF_ID {
		t.Errorf("Root leaf ID is the empty leaf ID")
	}
	if tree.Root.Leaf.AppId != "" {
		t.Errorf("Root leaf app ID is not empty")
	}
	if !tree.Root.Leaf.IsEmpty {
		t.Errorf("Root leaf is not marked as empty")
	}
}

func TestBTreeInsert(t *testing.T) {
	tree := NewTree(generaldata.Vector2i{X: 0, Y: 0})
	tree.AddApp("app1")
	if tree.LastFocusedContainer.AppId == "app1" {
		t.Errorf("Last focused app ID is not app1 and instead is %s", tree.LastFocusedContainer.AppId)
	}
}
