package jsondiff

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
)

type Difference int

const (
	FullMatch Difference = iota
	SupersetMatch
	NoMatch
	FirstArgIsInvalidJson
	SecondArgIsInvalidJson
	BothArgsAreInvalidJson
)

func (d Difference) String() string {
	switch d {
	case FullMatch:
		return "FullMatch"
	case SupersetMatch:
		return "SupersetMatch"
	case NoMatch:
		return "NoMatch"
	case FirstArgIsInvalidJson:
		return "FirstArgIsInvalidJson"
	case SecondArgIsInvalidJson:
		return "SecondArgIsInvalidJson"
	case BothArgsAreInvalidJson:
		return "BothArgsAreInvalidJson"
	}
	return "Invalid"
}

type Tag struct {
	Begin string
	End   string
}

type Options struct {
	Normal     Tag
	Added      Tag
	Removed    Tag
	Changed    Tag
	Prefix     string
	Indent     string
	PrintTypes bool
}

// Provides a set of options that are well suited for console output. Options
// use ANSI foreground color escape sequences to highlight changes.
func DefaultConsoleOptions() Options {
	return Options{
		Added:   Tag{Begin: "\033[0;32m", End: "\033[0m"},
		Removed: Tag{Begin: "\033[0;31m", End: "\033[0m"},
		Changed: Tag{Begin: "\033[0;33m", End: "\033[0m"},
		Indent:  "    ",
	}
}

// Provides a set of options that are well suited for HTML output. Works best
// inside <pre> tag.
func DefaultHTMLOptions() Options {
	return Options{
		Added:   Tag{Begin: `<span style="background-color: #8bff7f">`, End: `</span>`},
		Removed: Tag{Begin: `<span style="background-color: #fd7f7f">`, End: `</span>`},
		Changed: Tag{Begin: `<span style="background-color: #fcff7f">`, End: `</span>`},
		Indent:  "    ",
	}
}

type DiffMap struct {
	diff map[string][]string
}

func (d *DiffMap) GetAdded() []string {
	if v, ok := d.diff["Added"]; ok {
		return v
	}
	return make([]string, 0)
}

func (d *DiffMap) Added(v string) {
	d.diff["Added"] = append(d.diff["Added"], v)
}

func (d *DiffMap) Removed(v string) {
	d.diff["Removed"] = append(d.diff["Removed"], v)
}

func (d *DiffMap) Changed(v string) {
	d.diff["Changed"] = append(d.diff["Changed"], v)
}

func (d *DiffMap) GetRemoved() []string {
	if v, ok := d.diff["Removed"]; ok {
		return v
	}
	return make([]string, 0)
}

func (d *DiffMap) GetChanged() []string {
	if v, ok := d.diff["Changed"]; ok {
		return v
	}
	return make([]string, 0)
}
func NewDiffMap() DiffMap {
	d := DiffMap{diff: make(map[string][]string)}
	return d
}

type context struct {
	opts      *Options
	buf       bytes.Buffer
	level     int
	lastTag   *Tag
	diff      Difference
	diffMap   DiffMap
	keyString string
}

func (ctx *context) newline(s string) {
	ctx.buf.WriteString(s)
	if ctx.lastTag != nil {
		ctx.buf.WriteString(ctx.lastTag.End)
	}
	ctx.buf.WriteString("\n")
	ctx.buf.WriteString(ctx.opts.Prefix)
	for i := 0; i < ctx.level; i++ {
		ctx.buf.WriteString(ctx.opts.Indent)
	}
	if ctx.lastTag != nil {
		ctx.buf.WriteString(ctx.lastTag.Begin)
	}
}

func (ctx *context) key(k string) {
	ctx.keyString = k
	ctx.buf.WriteString(strconv.Quote(k))
	ctx.buf.WriteString(": ")
}

func (ctx *context) writeValue(v interface{}, full bool) {
	switch vv := v.(type) {
	case bool:
		ctx.buf.WriteString(strconv.FormatBool(vv))
	case json.Number:
		ctx.buf.WriteString(string(vv))
	case string:
		ctx.buf.WriteString(strconv.Quote(vv))
	case []interface{}:
		if full {
			if len(vv) == 0 {
				ctx.buf.WriteString("[")
			} else {
				ctx.level++
				ctx.newline("[")
			}
			for i, v := range vv {
				ctx.writeValue(v, true)
				if i != len(vv)-1 {
					ctx.newline(",")
				} else {
					ctx.level--
					ctx.newline("")
				}
			}
			ctx.buf.WriteString("]")
		} else {
			ctx.buf.WriteString("[]")
		}
	case map[string]interface{}:
		if full {
			if len(vv) == 0 {
				ctx.buf.WriteString("{")
			} else {
				ctx.level++
				ctx.newline("{")
			}
			i := 0
			for k, v := range vv {
				ctx.key(k)
				ctx.writeValue(v, true)
				if i != len(vv)-1 {
					ctx.newline(",")
				} else {
					ctx.level--
					ctx.newline("")
				}
				i++
			}
			ctx.buf.WriteString("}")
		} else {
			ctx.buf.WriteString("{}")
		}
	default:
		ctx.buf.WriteString("null")
	}

	ctx.writeTypeMaybe(v)
}

func (ctx *context) writeTypeMaybe(v interface{}) {
	if ctx.opts.PrintTypes {
		ctx.buf.WriteString(" ")
		ctx.writeType(v)
	}
}

func (ctx *context) writeType(v interface{}) {
	switch v.(type) {
	case bool:
		ctx.buf.WriteString("(boolean)")
	case json.Number:
		ctx.buf.WriteString("(number)")
	case string:
		ctx.buf.WriteString("(string)")
	case []interface{}:
		ctx.buf.WriteString("(array)")
	case map[string]interface{}:
		ctx.buf.WriteString("(object)")
	default:
		ctx.buf.WriteString("(null)")
	}
}

func (ctx *context) writeMismatch(a, b interface{}) {
	ctx.writeValue(a, false)
	ctx.buf.WriteString(" => ")
	// ctx.diffMap[a.(string)] = b.(string)
	ctx.writeValue(b, false)
}

func (ctx *context) tag(tag *Tag) {
	if ctx.lastTag == tag {
		return
	} else if ctx.lastTag != nil {
		ctx.buf.WriteString(ctx.lastTag.End)
	}
	ctx.buf.WriteString(tag.Begin)
	ctx.lastTag = tag
}

func (ctx *context) result(d Difference) {
	if d == NoMatch {
		ctx.diff = NoMatch
	} else if d == SupersetMatch && ctx.diff != NoMatch {
		ctx.diff = SupersetMatch
	} else if ctx.diff != NoMatch && ctx.diff != SupersetMatch {
		ctx.diff = FullMatch
	}
}

func (ctx *context) printMismatch(a, b interface{}) {
	ctx.tag(&ctx.opts.Changed)
	ctx.diffMap.Changed(ctx.keyString)
	ctx.writeMismatch(a, b)
}

func (ctx *context) printDiff(a, b interface{}) {
	if a == nil || b == nil {
		if a == nil && b == nil {
			ctx.tag(&ctx.opts.Normal)
			ctx.writeValue(a, false)
			ctx.result(FullMatch)
		} else {
			ctx.diffMap.Changed(ctx.keyString)
			ctx.printMismatch(a, b)
			ctx.result(NoMatch)
		}
		return
	}

	ka := reflect.TypeOf(a).Kind()
	kb := reflect.TypeOf(b).Kind()
	if ka != kb {
		ctx.printMismatch(a, b)
		ctx.result(NoMatch)
		return
	}
	switch ka {
	case reflect.Bool:
		if a.(bool) != b.(bool) {
			ctx.printMismatch(a, b)
			ctx.result(NoMatch)
			return
		}
	case reflect.String:
		switch aa := a.(type) {
		case json.Number:
			bb, ok := b.(json.Number)
			if !ok || aa != bb {
				ctx.printMismatch(a, b)
				ctx.result(NoMatch)
				return
			}
		case string:
			bb, ok := b.(string)
			if !ok || aa != bb {
				ctx.printMismatch(a, b)
				ctx.result(NoMatch)
				return
			}
		}
	case reflect.Slice:
		sa, sb := a.([]interface{}), b.([]interface{})
		salen, sblen := len(sa), len(sb)
		max := salen
		if sblen > max {
			max = sblen
		}
		ctx.tag(&ctx.opts.Normal)
		if max == 0 {
			ctx.buf.WriteString("[")
		} else {
			ctx.level++
			ctx.newline("[")
		}
		for i := 0; i < max; i++ {
			if i < salen && i < sblen {
				ctx.printDiff(sa[i], sb[i])
			} else if i < salen {
				ctx.diffMap.Removed(sa[i].(string))
				ctx.tag(&ctx.opts.Removed)
				ctx.writeValue(sa[i], true)
				ctx.result(SupersetMatch)
			} else if i < sblen {
				ctx.diffMap.Added(sb[i].(string))
				ctx.tag(&ctx.opts.Added)
				ctx.writeValue(sb[i], true)
				ctx.result(NoMatch)
			}
			ctx.tag(&ctx.opts.Normal)
			if i != max-1 {
				ctx.newline(",")
			} else {
				ctx.level--
				ctx.newline("")
			}
		}
		ctx.buf.WriteString("]")
		ctx.writeTypeMaybe(a)
		return
	case reflect.Map:
		ma, mb := a.(map[string]interface{}), b.(map[string]interface{})
		keysMap := make(map[string]bool)
		for k := range ma {
			keysMap[k] = true
		}
		for k := range mb {
			keysMap[k] = true
		}
		keys := make([]string, 0, len(keysMap))
		for k := range keysMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ctx.tag(&ctx.opts.Normal)
		if len(keys) == 0 {
			ctx.buf.WriteString("{")
		} else {
			ctx.level++
			ctx.newline("{")
		}
		for i, k := range keys {
			va, aok := ma[k]
			vb, bok := mb[k]
			if aok && bok {
				ctx.key(k)
				ctx.printDiff(va, vb)
			} else if aok {
				ctx.diffMap.Removed(k)
				ctx.tag(&ctx.opts.Removed)
				ctx.key(k)
				ctx.writeValue(va, true)
				ctx.result(SupersetMatch)
			} else if bok {
				ctx.diffMap.Added(k)
				ctx.tag(&ctx.opts.Added)
				ctx.key(k)
				ctx.writeValue(vb, true)
				ctx.result(NoMatch)
			}
			ctx.tag(&ctx.opts.Normal)
			if i != len(keys)-1 {
				ctx.newline(",")
			} else {
				ctx.level--
				ctx.newline("")
			}
		}
		ctx.buf.WriteString("}")
		ctx.writeTypeMaybe(a)
		return
	}
	ctx.tag(&ctx.opts.Normal)
	ctx.writeValue(a, true)
	ctx.result(FullMatch)
}

// Compares two JSON documents using given options. Returns difference type and
// a string describing differences.
//
// FullMatch means provided arguments are deeply equal.
//
// SupersetMatch means first argument is a superset of a second argument. In
// this context being a superset means that for each object or array in the
// hierarchy which don't match exactly, it must be a superset of another one.
// For example:
//
//     {"a": 123, "b": 456, "c": [7, 8, 9]}
//
// Is a superset of:
//
//     {"a": 123, "c": [7, 8]}
//
// NoMatch means there is no match.
//
// The rest of the difference types mean that one of or both JSON documents are
// invalid JSON.
//
// Returned string uses a format similar to pretty printed JSON to show the
// human-readable difference between provided JSON documents. It is important
// to understand that returned format is not a valid JSON and is not meant
// to be machine readable.
func Compare(a, b []byte, opts *Options) (Difference, string, DiffMap) {

	var av, bv interface{}
	da := json.NewDecoder(bytes.NewReader(a))
	da.UseNumber()
	db := json.NewDecoder(bytes.NewReader(b))
	db.UseNumber()
	errA := da.Decode(&av)
	errB := db.Decode(&bv)
	if errA != nil && errB != nil {
		return BothArgsAreInvalidJson, "both arguments are invalid json", NewDiffMap()
	}
	if errA != nil {
		return FirstArgIsInvalidJson, "first argument is invalid json", NewDiffMap()
	}
	if errB != nil {
		return SecondArgIsInvalidJson, "second argument is invalid json", NewDiffMap()
	}

	ctx := context{opts: opts,
		diffMap: NewDiffMap()}

	ctx.printDiff(av, bv)
	if ctx.lastTag != nil {
		ctx.buf.WriteString(ctx.lastTag.End)
	}
	return ctx.diff, ctx.buf.String(), ctx.diffMap
}
