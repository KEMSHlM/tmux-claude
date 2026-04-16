package gui

// TreeNodeKind distinguishes project-level from session-level nodes.
type TreeNodeKind int

const (
	ProjectNode TreeNodeKind = iota
	SessionNode
)

// ProjectItem is a read-only view of a project for display.
type ProjectItem struct {
	ID       string
	Name     string
	Path     string
	Host     string // SSH hostname derived from sessions (empty for local projects)
	Expanded bool
	Sessions []SessionItem // all sessions (PMs + workers); hierarchy is via ParentID
}

// TreeNode is a single row in the flattened tree view.
// The cursor indexes into a flat []TreeNode.
type TreeNode struct {
	Kind      TreeNodeKind
	ProjectID string
	Project   *ProjectItem // non-nil for ProjectNode
	Session   *SessionItem // non-nil for SessionNode
	Depth     int          // nesting depth (0 = project/root session, 1+ = children)
}

// BuildTreeNodes flattens projects into a list of TreeNodes with
// ParentID-based recursive hierarchy. pmCollapsed contains session IDs
// of PM nodes that are currently collapsed (children hidden).
//
// Tree structure:
//
//	Project → root sessions (ParentID=="") at depth 1
//	  PM node → children (ParentID==PM.ID) at depth 2+
//	    Worker / sub-PM → recursively deeper
func BuildTreeNodes(projects []ProjectItem, pmCollapsed map[string]bool) []TreeNode {
	var nodes []TreeNode
	for i := range projects {
		p := &projects[i]
		nodes = append(nodes, TreeNode{
			Kind:      ProjectNode,
			ProjectID: p.ID,
			Project:   p,
			Depth:     0,
		})
		if !p.Expanded {
			continue
		}
		// Build session ID set and index by ParentID for efficient child lookup.
		sessionIDs := make(map[string]bool, len(p.Sessions))
		childrenOf := make(map[string][]*SessionItem)
		for j := range p.Sessions {
			s := &p.Sessions[j]
			sessionIDs[s.ID] = true
			childrenOf[s.ParentID] = append(childrenOf[s.ParentID], s)
		}
		// Reparent orphans: sessions whose ParentID references a deleted/missing
		// session are promoted to root level so they remain visible in the tree.
		for pid, children := range childrenOf {
			if pid != "" && !sessionIDs[pid] {
				childrenOf[""] = append(childrenOf[""], children...)
				delete(childrenOf, pid)
			}
		}
		// Recursively append root sessions (ParentID=="") and their subtrees.
		appendSessionTree(&nodes, p.ID, childrenOf, "", 1, pmCollapsed)
	}
	return nodes
}

// maxTreeDepth caps recursion to prevent stack overflow from cyclic ParentID data.
const maxTreeDepth = 20

// appendSessionTree recursively appends sessions and their children to the
// node list. parentID identifies which children to add at this level;
// depth tracks the nesting for indentation.
func appendSessionTree(nodes *[]TreeNode, projectID string, childrenOf map[string][]*SessionItem, parentID string, depth int, pmCollapsed map[string]bool) {
	if depth > maxTreeDepth {
		return
	}
	children := childrenOf[parentID]
	for _, s := range children {
		// Heap-allocate a copy to avoid mutating the caller-owned slice element.
		item := new(SessionItem)
		*item = *s
		if item.Role == "pm" {
			item.Expanded = !pmCollapsed[item.ID]
		}
		*nodes = append(*nodes, TreeNode{
			Kind:      SessionNode,
			ProjectID: projectID,
			Session:   item,
			Depth:     depth,
		})
		// PM nodes can be collapsed to hide their children.
		if s.Role == "pm" && pmCollapsed[s.ID] {
			continue
		}
		// Recurse into children of this session (PM or otherwise).
		appendSessionTree(nodes, projectID, childrenOf, s.ID, depth+1, pmCollapsed)
	}
}
