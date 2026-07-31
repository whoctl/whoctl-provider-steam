package vdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// Binary VDF, the format shortcuts.vdf is written in.
//
// Steam uses the text syntax for everything a human might read and this tagged
// binary encoding for the files only the client touches. It is the same
// KeyValues tree underneath, so it decodes into the same Node type and the rest
// of the provider does not have to care which file it came from.
//
// The grammar is three type tags, a name terminated by NUL, and a payload:
//
//	0x00 name\0 <children…> 0x08   a nested map
//	0x01 name\0 value\0            a string
//	0x02 name\0 <int32 little-endian>
//	0x08                           end of the enclosing map
const (
	binMap    = 0x00
	binString = 0x01
	binInt32  = 0x02
	binEnd    = 0x08
)

// ReadBinaryFile parses a binary VDF file. A missing file yields a nil node and
// no error, matching ReadFile: a user who has never added a non-Steam game has
// no shortcuts.vdf, which is not a failure.
func ReadBinaryFile(path string) (*Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	nodes, err := ParseBinary(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return Root(nodes), nil
}

// ParseBinary decodes a binary VDF document.
func ParseBinary(data []byte) ([]*Node, error) {
	r := bytes.NewReader(data)
	nodes, err := readBinaryNodes(r, 0)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// maxBinaryDepth stops a corrupt or hostile file from driving the decoder into
// unbounded recursion; real shortcuts.vdf nests three levels.
const maxBinaryDepth = 32

func readBinaryNodes(r *bytes.Reader, depth int) ([]*Node, error) {
	if depth > maxBinaryDepth {
		return nil, fmt.Errorf("binary vdf nested deeper than %d levels", maxBinaryDepth)
	}
	var out []*Node
	for {
		tag, err := r.ReadByte()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if tag == binEnd {
			return out, nil
		}

		name, err := readCString(r)
		if err != nil {
			return nil, err
		}
		switch tag {
		case binMap:
			children, err := readBinaryNodes(r, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, &Node{Key: name, Block: true, Children: children})
		case binString:
			value, err := readCString(r)
			if err != nil {
				return nil, err
			}
			out = append(out, &Node{Key: name, Value: value})
		case binInt32:
			var v int32
			if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
				return nil, err
			}
			// Integers become strings so a Node reads the same whichever
			// encoding it came from; binaryInts remembers which to write back.
			out = append(out, &Node{Key: name, Value: strconv.FormatInt(int64(v), 10)})
		default:
			return nil, fmt.Errorf("unknown binary vdf tag 0x%02x", tag)
		}
	}
}

func readCString(r *bytes.Reader) (string, error) {
	var b []byte
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", fmt.Errorf("unterminated string: %w", err)
		}
		if c == 0 {
			return string(b), nil
		}
		b = append(b, c)
	}
}

// binaryInts are the shortcut fields Steam stores as int32 rather than as a
// string. Writing one back as a string produces a file the client silently
// discards, so the set has to be explicit — the Node tree itself cannot say
// which encoding a value had.
var binaryInts = map[string]bool{
	"appid":               true,
	"IsHidden":            true,
	"AllowDesktopConfig":  true,
	"AllowOverlay":        true,
	"OpenVR":              true,
	"Devkit":              true,
	"DevkitOverrideAppID": true,
	"LastPlayTime":        true,
}

// FormatBinary encodes nodes back to binary VDF.
func FormatBinary(nodes []*Node) ([]byte, error) {
	var buf bytes.Buffer
	for _, n := range nodes {
		if err := writeBinaryNode(&buf, n); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func writeBinaryNode(buf *bytes.Buffer, n *Node) error {
	if n.Block {
		buf.WriteByte(binMap)
		writeCString(buf, n.Key)
		for _, child := range n.Children {
			if err := writeBinaryNode(buf, child); err != nil {
				return err
			}
		}
		buf.WriteByte(binEnd)
		return nil
	}
	if binaryInts[n.Key] {
		v, err := strconv.ParseInt(n.Value, 10, 32)
		if err != nil {
			return fmt.Errorf("field %q must be a number, got %q", n.Key, n.Value)
		}
		buf.WriteByte(binInt32)
		writeCString(buf, n.Key)
		return binary.Write(buf, binary.LittleEndian, int32(v))
	}
	buf.WriteByte(binString)
	writeCString(buf, n.Key)
	writeCString(buf, n.Value)
	return nil
}

func writeCString(buf *bytes.Buffer, s string) {
	buf.WriteString(s)
	buf.WriteByte(0)
}

// WriteBinaryFile encodes a root node over its file, through a rename for the
// same reason the text writer does: a truncated shortcuts.vdf loses every
// non-Steam game the user ever added.
func WriteBinaryFile(path string, root *Node) error {
	if root == nil {
		return fmt.Errorf("refusing to write an empty document to %s", path)
	}
	data, err := FormatBinary([]*Node{root})
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".whoctl-*.vdf")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(name)
		if writeErr != nil {
			return fmt.Errorf("writing %s: %w", path, writeErr)
		}
		return fmt.Errorf("writing %s: %w", path, closeErr)
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
