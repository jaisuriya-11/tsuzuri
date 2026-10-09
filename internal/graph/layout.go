package graph

// The map is a sideways tree around one note: the notes it links to grow to
// the right, the notes linking to it grow to the left, and each further
// level adds the links (or backlinks) of the level before. Every note
// appears once, at the first level that reaches it.

// maxNodes keeps a densely linked workspace readable.
const maxNodes = 400

type gnode struct {
	id, title string
	side      int // -1 left (links here), 0 the centre, +1 right (links from)
	level     int
	parent    int // index of the node it hangs from, -1 for the centre
	kids      []int
	row       int
}

type tree struct {
	nodes    []gnode
	index    map[string]int
	rows     int // map height in rows
	minLevel int // the leftmost column, as -levels
	maxLevel int
}

// col is a node's column: 0 for the centre, negative to the left.
func (n gnode) col() int { return n.side * n.level }

// buildTree walks depth levels out from centre, links to the right and
// backlinks to the left, then gives every node a row: leaves stack step
// rows apart and each note sits level with the middle of its children.
func buildTree(centre string, titles map[string]string, out, in map[string][]string, depth, step int) tree {
	t := tree{index: map[string]int{}}
	add := func(id string, side, level, parent int) int {
		t.index[id] = len(t.nodes)
		t.nodes = append(t.nodes, gnode{id: id, title: titles[id], side: side, level: level, parent: parent})
		if parent >= 0 {
			t.nodes[parent].kids = append(t.nodes[parent].kids, len(t.nodes)-1)
		}
		return len(t.nodes) - 1
	}
	add(centre, 0, 0, -1)
	right, left := []int{0}, []int{0}
	for level := 1; level <= depth && len(t.nodes) < maxNodes; level++ {
		var nr, nl []int
		for _, p := range right {
			for _, id := range out[t.nodes[p].id] {
				if _, seen := t.index[id]; !seen && len(t.nodes) < maxNodes {
					nr = append(nr, add(id, 1, level, p))
				}
			}
		}
		for _, p := range left {
			for _, id := range in[t.nodes[p].id] {
				if _, seen := t.index[id]; !seen && len(t.nodes) < maxNodes {
					nl = append(nl, add(id, -1, level, p))
				}
			}
		}
		right, left = nr, nl
		if len(nr) > 0 {
			t.maxLevel = level
		}
		if len(nl) > 0 {
			t.minLevel = -level
		}
	}

	var place func(i int, next *int) int
	place = func(i int, next *int) int {
		kids := t.nodes[i].kids
		if len(kids) == 0 {
			t.nodes[i].row = *next
			*next += step
			return t.nodes[i].row
		}
		first := place(kids[0], next)
		last := first
		for _, k := range kids[1:] {
			last = place(k, next)
		}
		t.nodes[i].row = (first + last) / 2
		return t.nodes[i].row
	}
	side := func(s int) (root, height int) {
		next, first, last := 0, -1, -1
		for _, k := range t.nodes[0].kids {
			if t.nodes[k].side == s {
				r := place(k, &next)
				if first < 0 {
					first = r
				}
				last = r
			}
		}
		if first < 0 {
			return 0, 1
		}
		return (first + last) / 2, next - step + 1
	}
	rootR, hR := side(1)
	rootL, hL := side(-1)
	centreRow := max(rootR, rootL)
	for i := range t.nodes[1:] {
		n := &t.nodes[i+1]
		if n.side > 0 {
			n.row += centreRow - rootR
		} else {
			n.row += centreRow - rootL
		}
	}
	t.nodes[0].row = centreRow
	t.rows = max(hR+centreRow-rootR, hL+centreRow-rootL, centreRow+1)
	return t
}

// onPath reports whether node i lies between node sel and the centre.
func (t tree) onPath(i, sel int) bool {
	for j := sel; j >= 0; j = t.nodes[j].parent {
		if j == i {
			return true
		}
	}
	return false
}
