package main

// acNode is a node in the Aho-Corasick automaton.
type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	output   []string // pattern keys that end at this node
}

// AhoCorasick is the compiled multi-pattern matcher.
type AhoCorasick struct {
	root *acNode
}

// NewAhoCorasick builds the automaton from the given pattern keys.
func NewAhoCorasick(patterns []string) *AhoCorasick {
	root := &acNode{children: make(map[rune]*acNode)}

	// Phase 1: build trie
	for _, pat := range patterns {
		cur := root
		for _, ch := range pat {
			if cur.children[ch] == nil {
				cur.children[ch] = &acNode{children: make(map[rune]*acNode)}
			}
			cur = cur.children[ch]
		}
		cur.output = append(cur.output, pat)
	}

	// Phase 2: build failure links via BFS
	queue := make([]*acNode, 0, len(root.children))
	for _, child := range root.children {
		child.fail = root
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for ch, child := range cur.children {
			fail := cur.fail
			for fail != nil && fail.children[ch] == nil {
				fail = fail.fail
			}
			if fail == nil {
				child.fail = root
			} else {
				child.fail = fail.children[ch]
				if child.fail == child {
					child.fail = root
				}
			}
			// merge outputs from fail chain
			child.output = append(child.output, child.fail.output...)
			queue = append(queue, child)
		}
	}

	return &AhoCorasick{root: root}
}

// Search returns all pattern keys found in text (may have duplicates if the same pattern
// is found at multiple positions).
func (ac *AhoCorasick) Search(text string) []string {
	cur := ac.root
	var matches []string
	for _, ch := range text {
		for cur != ac.root && cur.children[ch] == nil {
			cur = cur.fail
		}
		if next, ok := cur.children[ch]; ok {
			cur = next
		}
		matches = append(matches, cur.output...)
	}
	return matches
}
