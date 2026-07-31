// Package vdf reads and writes Valve Data Format, the KeyValues syntax every
// Steam configuration file is written in.
//
// The format is small: a quoted key followed by either a quoted value or a
// braced block of more keys. Comments start with //, keys may repeat, and order
// is significant to Steam in some files, so a Node keeps its children in the
// order they were read rather than in a map.
//
// Writing preserves everything the model does not touch, the same rule the
// linux provider follows for /etc/resolv.conf and the repository files: a Steam
// config holds hundreds of keys whoctl has no opinion about, and rewriting it
// from a model would throw all of them away.
package vdf

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Node is one entry: either a leaf with a Value, or a block with Children.
type Node struct {
	Key      string
	Value    string
	Children []*Node
	// Block distinguishes an empty block from an empty string value, which are
	// written differently and mean different things.
	Block bool
}

// Parse reads a VDF document and returns its top-level nodes. Most Steam files
// have exactly one, named after the store they represent.
func Parse(r io.Reader) ([]*Node, error) {
	p := &parser{s: bufio.NewReader(r)}
	nodes, err := p.parseNodes(true)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// ParseString is Parse over a string.
func ParseString(s string) ([]*Node, error) { return Parse(strings.NewReader(s)) }

// Root returns the first top-level node, which is what nearly every Steam file
// has exactly one of.
func Root(nodes []*Node) *Node {
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

// Find walks a path of keys and returns the node at the end, or nil.
// Lookups are case-insensitive because Steam is inconsistent about casing
// between client versions — "apps" and "Apps" both occur in the wild.
func (n *Node) Find(path ...string) *Node {
	current := n
	for _, key := range path {
		if current == nil {
			return nil
		}
		var next *Node
		for _, child := range current.Children {
			if strings.EqualFold(child.Key, key) {
				next = child
				break
			}
		}
		current = next
	}
	return current
}

// Get returns the string value at a path, or "" when it is absent.
func (n *Node) Get(path ...string) string {
	if found := n.Find(path...); found != nil {
		return found.Value
	}
	return ""
}

// GetInt returns the integer value at a path. Steam writes every number as a
// quoted string, so a missing or unparsable value is reported as zero and false
// rather than as an error.
func (n *Node) GetInt(path ...string) (int64, bool) {
	raw := strings.TrimSpace(n.Get(path...))
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// GetBool reads Steam's 0/1 flags.
func (n *Node) GetBool(path ...string) bool {
	v, ok := n.GetInt(path...)
	return ok && v != 0
}

// Set writes a leaf value at a path, creating the blocks along the way and
// leaving every sibling untouched. This is what makes a rewrite non-destructive.
func (n *Node) Set(value string, path ...string) {
	if len(path) == 0 {
		return
	}
	parent := n.ensure(path[:len(path)-1]...)
	key := path[len(path)-1]
	for _, child := range parent.Children {
		if strings.EqualFold(child.Key, key) {
			child.Value, child.Children, child.Block = value, nil, false
			return
		}
	}
	parent.Children = append(parent.Children, &Node{Key: key, Value: value})
}

// Delete removes the node at a path and reports whether it was there.
func (n *Node) Delete(path ...string) bool {
	if len(path) == 0 {
		return false
	}
	parent := n.Find(path[:len(path)-1]...)
	if parent == nil {
		return false
	}
	key := path[len(path)-1]
	for i, child := range parent.Children {
		if strings.EqualFold(child.Key, key) {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return true
		}
	}
	return false
}

// ensure returns the block at a path, creating what is missing.
func (n *Node) ensure(path ...string) *Node {
	current := n
	for _, key := range path {
		next := current.Find(key)
		if next == nil {
			next = &Node{Key: key, Block: true}
			current.Children = append(current.Children, next)
		}
		// A leaf standing where a block is needed becomes one; Steam writes an
		// empty string for a block it has never populated.
		next.Block = true
		current = next
	}
	return current
}

// Format renders nodes back to VDF text, using the tab indentation and the
// double-tab separator between key and value that Steam itself writes.
func Format(nodes []*Node) string {
	var b strings.Builder
	for _, n := range nodes {
		writeNode(&b, n, 0)
	}
	return b.String()
}

func writeNode(b *strings.Builder, n *Node, depth int) {
	indent := strings.Repeat("\t", depth)
	if !n.Block {
		fmt.Fprintf(b, "%s%s\t\t%s\n", indent, quote(n.Key), quote(n.Value))
		return
	}
	fmt.Fprintf(b, "%s%s\n%s{\n", indent, quote(n.Key), indent)
	for _, child := range n.Children {
		writeNode(b, child, depth+1)
	}
	fmt.Fprintf(b, "%s}\n", indent)
}

// quote escapes the two characters VDF gives meaning to inside a quoted string.
func quote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}

type parser struct {
	s *bufio.Reader
}

// parseNodes reads nodes until EOF (at the top level) or a closing brace.
func (p *parser) parseNodes(top bool) ([]*Node, error) {
	var out []*Node
	for {
		tok, ok, err := p.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			if top {
				return out, nil
			}
			return nil, fmt.Errorf("unexpected end of file inside a block")
		}
		if tok == "}" {
			if top {
				return nil, fmt.Errorf("unexpected }")
			}
			return out, nil
		}
		if tok == "{" {
			return nil, fmt.Errorf("unexpected { where a key was expected")
		}

		node := &Node{Key: tok}
		valueTok, ok, err := p.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			// A key with nothing after it is how some Steam files end; treat it
			// as an empty value rather than failing the whole read.
			out = append(out, node)
			return out, nil
		}
		switch valueTok {
		case "{":
			node.Block = true
			children, err := p.parseNodes(false)
			if err != nil {
				return nil, err
			}
			node.Children = children
		case "}":
			return nil, fmt.Errorf("key %q has no value", node.Key)
		default:
			node.Value = valueTok
		}
		out = append(out, node)
	}
}

// next returns the next token: a brace, or the contents of a quoted or bare
// string. ok is false at end of file.
func (p *parser) next() (string, bool, error) {
	for {
		r, _, err := p.s.ReadRune()
		if err == io.EOF {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		switch {
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			continue
		case r == '{' || r == '}':
			return string(r), true, nil
		case r == '/':
			// Comments are // to end of line. A lone / is not valid VDF, but
			// skipping to the newline is the forgiving reading.
			if err := p.skipLine(); err != nil {
				return "", false, err
			}
			continue
		case r == '"':
			s, err := p.quoted()
			return s, true, err
		default:
			if err := p.s.UnreadRune(); err != nil {
				return "", false, err
			}
			s, err := p.bare()
			return s, true, err
		}
	}
}

func (p *parser) quoted() (string, error) {
	var b strings.Builder
	for {
		r, _, err := p.s.ReadRune()
		if err != nil {
			return "", fmt.Errorf("unterminated string: %w", err)
		}
		switch r {
		case '"':
			return b.String(), nil
		case '\\':
			esc, _, err := p.s.ReadRune()
			if err != nil {
				return "", fmt.Errorf("unterminated escape: %w", err)
			}
			switch esc {
			case 'n':
				b.WriteRune('\n')
			case 't':
				b.WriteRune('\t')
			case '\\', '"':
				b.WriteRune(esc)
			default:
				// VDF has no other escapes; keep the pair verbatim so a
				// Windows path such as "C:\Program Files" survives a rewrite.
				b.WriteRune('\\')
				b.WriteRune(esc)
			}
		default:
			b.WriteRune(r)
		}
	}
}

// bare reads an unquoted token, which appears in older hand-edited files.
func (p *parser) bare() (string, error) {
	var b strings.Builder
	for {
		r, _, err := p.s.ReadRune()
		if err == io.EOF {
			return b.String(), nil
		}
		if err != nil {
			return "", err
		}
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == '{' || r == '}' {
			if r == '{' || r == '}' {
				if err := p.s.UnreadRune(); err != nil {
					return "", err
				}
			}
			return b.String(), nil
		}
		b.WriteRune(r)
	}
}

func (p *parser) skipLine() error {
	for {
		r, _, err := p.s.ReadRune()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if r == '\n' {
			return nil
		}
	}
}
