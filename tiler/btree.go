package tiler

import (
	"errors"
	"fmt"
	"math"
	"sync"

	generaldata "github.com/mstarongithub/way2gay/general-data"
)

type NodeType int
type Direction int

const (
	NodeTypeLeaf = NodeType(iota)
	NodeTypeBranch
)

const EMPTY_LEAF_ID = 0

const (
	DirectionVertical = Direction(iota)
	DirectionHorizontal
)

type (
	// A tiling tree. One tree per screen/workspace
	// Children resolution calculated down the tree
	Tree struct {
		Resolution           generaldata.Vector2i // Final space the tree is occupying
		nameToId             map[string]int       // Stores all leaflet IDs for quick lookup
		Root                 Node
		LastFocusedContainer *Leaf
		LastFocusedParent    *Branch
		lock                 sync.Mutex
	}

	// Wrappe container for leafs, branches or empty nodes
	Node struct {
		Type   NodeType
		Branch *Branch // Must be set if type is NodeTypeBranch, ignored otherwise
		Leaf   *Leaf   // Must be set if type is NodeTypeLeaf, ignored otherwise
	}

	Branch struct {
		Direction  Direction // TODO: Is this needed or can it be inferred from parents?
		ChildLeft  Node      // Is the top child if split vertically
		ChildRight Node      // Is the bottom child if split vertically
		AspectLeft int       // Percentage the left child has of the container space

		// Range of IDs the leaflets have. Includes sub-branches
		idRangeStart int // Start is inclusive
		idRangeEnd   int // End is exclusive
	}

	Leaf struct {
		leafID  int    // Unique ID for this leaf. ONLY CHANGE WHEN INSERTING NEW LEAFS AND ON CHANGE ALSO UPDATE THE MAPPING IN THE TREE ROOT
		AppId   string // TODO: Should be a reference to the app contained
		IsEmpty bool   // Indicates that this leaf is empty
	}

	LeafNeighbours struct {
		Up    *Leaf
		Down  *Leaf
		Left  *Leaf
		Right *Leaf
	}
)

func NewTree(resolution generaldata.Vector2i) Tree {
	baseLeaf := Leaf{
		leafID:  EMPTY_LEAF_ID,
		AppId:   "",
		IsEmpty: true,
	}
	baseNode := Node{
		Type:   NodeTypeLeaf,
		Branch: nil,
		Leaf:   &baseLeaf,
	}
	return Tree{
		Resolution: resolution,
		nameToId: map[string]int{
			"": EMPTY_LEAF_ID, // Empty appID will always map to the empty leaf. Please don't overwrite
		},
		Root:                 baseNode,
		LastFocusedContainer: &baseLeaf,
		LastFocusedParent:    nil,
	}
}

// Find the leaflet containing the given app
func (t *Tree) FindApp(appId string) *Leaf {
	t.lock.Lock()
	defer t.lock.Unlock()

	uID, ok := t.nameToId[appId]
	if !ok {
		// Case app not in tree
		return nil
	}
	switch t.Root.Type {
	case NodeTypeLeaf:
		// Only one leaf stored
		if t.Root.Leaf.leafID == uID {
			return t.Root.Leaf
		}
	case NodeTypeBranch:
		return t.Root.Branch.findUID(uID)
	}

	return nil
}

func (t *Tree) findAndTrace(appId string) (*Leaf, []Node) {
	t.lock.Lock()
	defer t.lock.Unlock()

	uID, ok := t.nameToId[appId]
	if !ok {
		// Case app not in tree
		return nil, []Node{}
	}
	switch t.Root.Type {
	case NodeTypeLeaf:
		// Only one leaf stored
		if t.Root.Leaf.leafID == uID {
			return t.Root.Leaf, []Node{t.Root}
		}
	case NodeTypeBranch:
		return t.Root.Branch.findAndTrace(uID)
	}

	return nil, []Node{}
}

// Swap two apps
func (t *Tree) SwapApp(app1, app2 string) {
	t.lock.Lock()
	defer t.lock.Unlock()

	leaflet1 := t.FindApp(app1)
	leaflet2 := t.FindApp(app2)

	if leaflet1 == nil || leaflet2 == nil {
		return
	}

	// Don't swap leaf IDs. That would cause problems with search
	// Store data of leaflet1
	tempName := leaflet1.AppId
	tempID := leaflet1.leafID
	// Then swap IDs in the name to ID map
	t.nameToId[app1] = leaflet2.leafID
	t.nameToId[app2] = tempID
	// Then the names
	leaflet1.AppId = leaflet2.AppId
	leaflet2.AppId = tempName
}

// Add a new app to the tree
// Will split the last focused container if needed
func (t *Tree) AddApp(appId string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.SplitLastFocusedContainer()
	newLeaf := Leaf{
		leafID:  t.LastFocusedParent.idRangeEnd - 1,
		AppId:   appId,
		IsEmpty: false,
	}
	t.nameToId[appId] = newLeaf.leafID
	t.LastFocusedParent.ChildRight = Node{
		Type: NodeTypeLeaf,
		Leaf: &newLeaf,
	}
	t.LastFocusedContainer = &newLeaf
}

// Remove an app from the tree
// If popParent is true, the parent container will be removed and replaced with the other child
func (t *Tree) RemoveApp(appId string, popParent bool) {
	t.lock.Lock()
	defer t.lock.Unlock()

	leaf, trace := t.findAndTrace(appId)
	if leaf == nil {
		// Didn't find app, nothing to do
		return
	}
	// 1. Remove app from name to ID map
	delete(t.nameToId, appId)

	// 2. Set app leaflet to empty
	leaf.IsEmpty = true
	leaf.AppId = ""
	leaf.leafID = EMPTY_LEAF_ID
	// 3. If told to pop parent, remove parent branch and replace with other child
	// But only do so if not top level
	if popParent && len(trace) > 1 {
		parent := trace[1]

		// Check if parent is top level
		if parent.Branch == t.Root.Branch {
			// Case: Parent is top level
			if parent.Branch.ChildLeft.Leaf.leafID == leaf.leafID {
				t.Root = parent.Branch.ChildRight
			} else {
				t.Root = parent.Branch.ChildLeft
			}
			// And GC should clean up the old branch and leaf to delete
		} else {
			// Technically there is an edge case here where the other child is empty node
			// Doesn't matter though since empty node is empty node
			parentsParent := trace[2]
			if parentsParent.Branch.ChildLeft.Branch == parent.Branch {
				// Parent is left child
				if parent.Branch.ChildLeft.Leaf.leafID == leaf.leafID {
					parentsParent.Branch.ChildLeft = parent.Branch.ChildRight
				} else {
					parentsParent.Branch.ChildLeft = parent.Branch.ChildLeft
				}
			} else {
				// Parent is right child
				if parent.Branch.ChildLeft.Leaf.leafID == leaf.leafID {
					parentsParent.Branch.ChildRight = parent.Branch.ChildRight
				} else {
					parentsParent.Branch.ChildRight = parent.Branch.ChildLeft
				}
			}
		}
		// And again, GC should clean up the old branch and leaf to delete
	}
}

// Split the last focused container into a new branch
// the container itself will be placed as the left child of the new branch
// Right side will be an empty leaf
// Updates leaf IDs if needed
func (t *Tree) SplitLastFocusedContainer() {
	// Case: No last focused parent -> Working at top level
	if t.LastFocusedParent == nil {
		// First make a new split container
		newSplit := Branch{
			// Default to vertical for initial split
			// TODO: Make this configurable
			Direction: DirectionVertical,
			// First split container will always be max range
			idRangeStart: math.MinInt,
			idRangeEnd:   math.MaxInt,
			// Left entry is last focused leaf
			ChildLeft: t.Root,
			// Right entry is empty
			ChildRight: Node{
				Type: NodeTypeLeaf,
				Leaf: &Leaf{
					leafID:  EMPTY_LEAF_ID,
					IsEmpty: true,
				},
			},
		}
		// Then update root
		t.Root = Node{
			Type:   NodeTypeBranch,
			Branch: &newSplit,
		}
		t.LastFocusedParent = &newSplit
		t.LastFocusedContainer = t.Root.Branch.ChildLeft.Leaf
	} else {
		// Todo: Package last focused node into container, replace it with new split, then set left container of new split to last focused and update root
		// 1. Package last focused node into container
		packagedLeaf := Node{
			Type: NodeTypeLeaf,
			Leaf: t.LastFocusedContainer,
		}
		// 2. Create new branch
		newDirection := DirectionVertical
		if t.LastFocusedParent.Direction == DirectionVertical {
			newDirection = DirectionHorizontal
		}
		// New range start is same as parent range start if left side, else it's the middle
		newRangeStart := t.LastFocusedParent.idRangeStart
		if t.LastFocusedContainer.leafID == t.LastFocusedParent.ChildLeft.Leaf.leafID {
			// Left side: Copy from parent
			newRangeStart = t.LastFocusedParent.idRangeStart
		} else {
			// Right side: Start is middle
			newRangeStart = t.LastFocusedParent.idRangeStart + ((t.LastFocusedParent.idRangeEnd - t.LastFocusedParent.idRangeStart) / 2)
		}
		// Same here, except middle if left, else end
		newRangeEnd := t.LastFocusedParent.idRangeEnd
		if t.LastFocusedContainer.leafID == t.LastFocusedParent.ChildRight.Leaf.leafID {
			// Right side: Copy from parent
			newRangeEnd = t.LastFocusedParent.idRangeEnd
		} else {
			// Left side: End is middle
			newRangeEnd = t.LastFocusedParent.idRangeStart + ((t.LastFocusedParent.idRangeEnd - t.LastFocusedParent.idRangeStart) / 2)
		}

		newBranch := Branch{
			Direction: newDirection,
			ChildLeft: packagedLeaf,
			ChildRight: Node{
				Type: NodeTypeLeaf,
				Leaf: &Leaf{
					leafID:  EMPTY_LEAF_ID,
					IsEmpty: true,
				},
			},
			AspectLeft:   50,
			idRangeStart: newRangeStart,
			idRangeEnd:   newRangeEnd,
		}

		// 3. Replace self in parent branch with new branch
		if t.LastFocusedParent.ChildLeft.Leaf.leafID == t.LastFocusedContainer.leafID {
			t.LastFocusedParent.ChildLeft = Node{
				Type:   NodeTypeBranch,
				Branch: &newBranch,
			}
		} else {
			t.LastFocusedParent.ChildRight = Node{
				Type:   NodeTypeBranch,
				Branch: &newBranch,
			}
		}

		// 4. Update root
		t.LastFocusedParent = &newBranch

		// 5. Update ID if needed
		// Edgecase of parent range being one element should be fixed
		if t.LastFocusedContainer.leafID > t.LastFocusedParent.idRangeEnd {
			t.LastFocusedContainer.leafID = t.LastFocusedParent.idRangeEnd - 1
		} else if t.LastFocusedContainer.leafID < t.LastFocusedParent.idRangeStart {
			t.LastFocusedContainer.leafID = t.LastFocusedParent.idRangeStart
		}
	}
}

// Recursively find the leaflet with the given ID
func (b *Branch) findUID(uID int) *Leaf {
	if b.ChildLeft.Type == NodeTypeLeaf && b.ChildLeft.Leaf.leafID == uID {
		return b.ChildLeft.Leaf
	}
	if b.ChildRight.Type == NodeTypeLeaf && b.ChildRight.Leaf.leafID == uID {
		return b.ChildRight.Leaf
	}
	if b.ChildLeft.Type == NodeTypeBranch && uID >= b.ChildLeft.Branch.idRangeStart && uID < b.ChildLeft.Branch.idRangeEnd {
		return b.ChildLeft.Branch.findUID(uID)
	}
	if b.ChildRight.Type == NodeTypeBranch && uID >= b.ChildRight.Branch.idRangeStart && uID < b.ChildRight.Branch.idRangeEnd {
		return b.ChildRight.Branch.findUID(uID)
	}
	return nil
}

// Same as findUID but also returns the trace of nodes leading to the leaflet
// The order of the trace is from the leaflet to the root
// Trace is empty if the leaflet is not found
func (b *Branch) findAndTrace(uID int) (*Leaf, []Node) {
	if b.ChildLeft.Type == NodeTypeLeaf && b.ChildLeft.Leaf.leafID == uID {
		return b.ChildLeft.Leaf, []Node{b.ChildLeft}
	}
	if b.ChildRight.Type == NodeTypeLeaf && b.ChildRight.Leaf.leafID == uID {
		return b.ChildRight.Leaf, []Node{b.ChildRight}
	}
	if b.ChildLeft.Type == NodeTypeBranch && uID >= b.ChildLeft.Branch.idRangeStart && uID < b.ChildLeft.Branch.idRangeEnd {
		leaf, trace := b.ChildLeft.Branch.findAndTrace(uID)
		trace = append(trace, Node{Type: NodeTypeBranch, Branch: b})
		return leaf, trace
	}
	if b.ChildRight.Type == NodeTypeBranch && uID >= b.ChildRight.Branch.idRangeStart && uID < b.ChildRight.Branch.idRangeEnd {
		leaf, trace := b.ChildRight.Branch.findAndTrace(uID)
		trace = append(trace, Node{Type: NodeTypeBranch, Branch: b})
		return leaf, trace
	}
	return nil, []Node{}
}

func checkNode(node *Node, rangeStart, rangeEnd int) error {
	if node == nil {
		return errors.New("node is nil")
	}
	if node.Type == NodeTypeBranch {
		return checkBranch(node)
	} else if node.Type == NodeTypeLeaf {
		return checkLeaf(node, rangeStart, rangeEnd)
	}

	return errors.New("invalid node type")
}

func checkBranch(node *Node) error {
	if node == nil {
		return errors.New("node is nil")
	}
	if node.Type != NodeTypeBranch {
		return errors.New("node not a branch")
	}
	if node.Branch == nil {
		return errors.New("stored branch is nil")
	}
	if node.Branch.idRangeEnd < node.Branch.idRangeStart {
		return fmt.Errorf("invalid ID range: %d - %d", node.Branch.idRangeStart, node.Branch.idRangeEnd)
	}

	if err := checkNode(&node.Branch.ChildLeft, node.Branch.idRangeStart, node.Branch.idRangeEnd); err != nil {
		return fmt.Errorf("Left child: %w", err)
	}

	if err := checkNode(&node.Branch.ChildRight, node.Branch.idRangeStart, node.Branch.idRangeEnd); err != nil {
		return fmt.Errorf("Left child: %w", err)
	}

	return nil
}

func checkLeaf(node *Node, rangeStart, rangeEnd int) error {
	if node == nil {
		return errors.New("node is nil")
	}
	if node.Type != NodeTypeLeaf {
		return errors.New("node not a leaf")
	}
	if node.Leaf == nil {
		return errors.New("leaf is nil")
	}
	if node.Leaf.leafID < rangeStart || node.Leaf.leafID >= rangeEnd {
		return fmt.Errorf("leaf ID %v out of range (%v, %v)", node.Leaf.leafID, rangeStart, rangeEnd)
	}

	return nil
}
