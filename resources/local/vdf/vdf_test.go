package vdf

import (
	"strings"
	"testing"
)

const sample = `// a comment Steam sometimes leaves at the top
"InstallConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"CompatToolMapping"
				{
					"620"
					{
						"name"		"GE-Proton9-20"
						"priority"		"250"
					}
				}
				"WindowsPath"		"C:\\Program Files\\Steam"
				"Quoted"		"he said \"hi\""
				"EmptyBlock"
				{
				}
			}
		}
	}
}
`

func TestParseReadsTheTree(t *testing.T) {
	root, err := ParseString(sample)
	if err != nil {
		t.Fatal(err)
	}
	r := Root(root)
	if r.Key != "InstallConfigStore" {
		t.Fatalf("root = %q", r.Key)
	}
	if got := r.Get("Software", "Valve", "Steam", "CompatToolMapping", "620", "name"); got != "GE-Proton9-20" {
		t.Errorf("name = %q", got)
	}
	if v, ok := r.GetInt("Software", "Valve", "Steam", "CompatToolMapping", "620", "priority"); !ok || v != 250 {
		t.Errorf("priority = %d, %v", v, ok)
	}
	if r.Find("Software", "Valve", "Steam", "Nothing") != nil {
		t.Error("a missing path must yield nil")
	}
}

func TestParseKeepsWindowsPathsAndQuotes(t *testing.T) {
	r := Root(mustParse(t, sample))
	steam := r.Find("Software", "Valve", "Steam")
	// VDF has no \P escape, so the backslash pair has to survive as written or
	// a Windows path is mangled on the way back out.
	if got := steam.Get("WindowsPath"); got != `C:\Program Files\Steam` {
		t.Errorf("WindowsPath = %q", got)
	}
	if got := steam.Get("Quoted"); got != `he said "hi"` {
		t.Errorf("Quoted = %q", got)
	}
}

func TestFormatIsIdempotentAndReparsable(t *testing.T) {
	nodes := mustParse(t, sample)
	once := Format(nodes)
	twice := Format(mustParse(t, once))
	if once != twice {
		t.Errorf("formatting is not stable:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
	// The escapes have to round-trip, not just survive one direction.
	r := Root(mustParse(t, once))
	if got := r.Get("Software", "Valve", "Steam", "WindowsPath"); got != `C:\Program Files\Steam` {
		t.Errorf("after a round trip WindowsPath = %q", got)
	}
}

func TestFormatKeepsAnEmptyBlockABlock(t *testing.T) {
	// An empty block and an empty string value are written differently and mean
	// different things to Steam.
	out := Format(mustParse(t, sample))
	if !strings.Contains(out, "\"EmptyBlock\"\n") {
		t.Errorf("the empty block was written as a value:\n%s", out)
	}
}

func TestSetCreatesTheBlocksAlongTheWay(t *testing.T) {
	r := Root(mustParse(t, sample))
	r.Set("proton_experimental", "Software", "Valve", "Steam", "CompatToolMapping", "570", "name")
	if got := r.Get("Software", "Valve", "Steam", "CompatToolMapping", "570", "name"); got != "proton_experimental" {
		t.Errorf("name = %q", got)
	}
	// Everything that was already there has to still be there.
	if got := r.Get("Software", "Valve", "Steam", "CompatToolMapping", "620", "name"); got != "GE-Proton9-20" {
		t.Errorf("the sibling entry was lost: %q", got)
	}
}

func TestSetReplacesRatherThanAppends(t *testing.T) {
	r := Root(mustParse(t, sample))
	r.Set("proton_9", "Software", "Valve", "Steam", "CompatToolMapping", "620", "name")
	mapping := r.Find("Software", "Valve", "Steam", "CompatToolMapping", "620")
	count := 0
	for _, child := range mapping.Children {
		if child.Key == "name" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the key was written %d times, want 1", count)
	}
}

func TestDeleteRemovesOnlyItsOwnKey(t *testing.T) {
	r := Root(mustParse(t, sample))
	if !r.Delete("Software", "Valve", "Steam", "CompatToolMapping", "620", "priority") {
		t.Fatal("Delete reported nothing removed")
	}
	if r.Get("Software", "Valve", "Steam", "CompatToolMapping", "620", "name") == "" {
		t.Error("the sibling key was removed too")
	}
	if r.Delete("Software", "Valve", "Steam", "NotThere") {
		t.Error("Delete reported removing something that was not there")
	}
}

func TestLookupIsCaseInsensitive(t *testing.T) {
	// Steam is inconsistent about casing between client versions; "apps" and
	// "Apps" both occur in files in the wild.
	r := Root(mustParse(t, sample))
	if r.Find("software", "valve", "steam") == nil {
		t.Error("a lowercase path must resolve")
	}
}

func TestParseRejectsBrokenDocuments(t *testing.T) {
	for _, broken := range []string{`"key" }`, `}`, `"a" { "b" "c"`} {
		if _, err := ParseString(broken); err == nil {
			t.Errorf("ParseString(%q) accepted it", broken)
		}
	}
}

// --- binary ------------------------------------------------------------

func TestBinaryRoundTrip(t *testing.T) {
	// The shape of a real shortcuts.vdf: a map of numbered entries, each with
	// int32 and string fields plus a nested tags map.
	shortcuts := &Node{Key: "shortcuts", Block: true, Children: []*Node{
		{Key: "0", Block: true, Children: []*Node{
			{Key: "appid", Value: "-1234567"},
			{Key: "AppName", Value: "Lutris"},
			{Key: "Exe", Value: `"/usr/bin/lutris"`},
			{Key: "LaunchOptions", Value: ""},
			{Key: "IsHidden", Value: "0"},
			{Key: "tags", Block: true, Children: []*Node{
				{Key: "0", Value: "Emulators"},
			}},
		}},
	}}

	data, err := FormatBinary([]*Node{shortcuts})
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseBinary(data)
	if err != nil {
		t.Fatal(err)
	}
	r := Root(back)
	if got := r.Get("0", "AppName"); got != "Lutris" {
		t.Errorf("AppName = %q", got)
	}
	if got := r.Get("0", "Exe"); got != `"/usr/bin/lutris"` {
		t.Errorf("Exe = %q", got)
	}
	if got := r.Get("0", "tags", "0"); got != "Emulators" {
		t.Errorf("tag = %q", got)
	}
}

func TestBinaryWritesIntegersAsIntegers(t *testing.T) {
	// A field Steam stores as int32 must go back as int32; written as a string
	// the client silently discards the whole shortcut.
	data, err := FormatBinary([]*Node{{Key: "shortcuts", Block: true, Children: []*Node{
		{Key: "0", Block: true, Children: []*Node{
			{Key: "appid", Value: "7"},
			{Key: "AppName", Value: "x"},
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	// 0x02 is the int32 tag; it has to precede "appid".
	idx := strings.Index(string(data), "appid")
	if idx < 1 || data[idx-1] != 0x02 {
		t.Errorf("appid was not written with the int32 tag: % x", data)
	}
	// AppName is a string, tag 0x01.
	nameIdx := strings.Index(string(data), "AppName")
	if nameIdx < 1 || data[nameIdx-1] != 0x01 {
		t.Errorf("AppName was not written with the string tag: % x", data)
	}
}

func TestBinaryRejectsGarbage(t *testing.T) {
	if _, err := ParseBinary([]byte{0x7f, 'x', 0}); err == nil {
		t.Error("an unknown tag must be an error rather than a silent skip")
	}
}

func mustParse(t *testing.T, s string) []*Node {
	t.Helper()
	nodes, err := ParseString(s)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	return nodes
}
