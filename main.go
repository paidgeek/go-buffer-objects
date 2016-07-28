package main

import (
	"flag"
	"os"
	"log"
	"io/ioutil"
	"text/template"
	"fmt"
	"unicode"
	"strings"
	"strconv"
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"github.com/paidgeek/bufobjects/bindata"
	"gopkg.in/ini.v1"
	"encoding/json"
	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/hashset"
)

type field struct {
	Name      string
	CamelCase string
	Type      string
}

type object struct {
	Id      uint16
	Name    string
	RawName string
	Fields  []*field
}

type document struct {
	MaxObjectSize    int `json:"max_object_size"`
	PackageName      string `json:"package_name"`
	ObjectsImpl      string
	ObjectNameSuffix string `json:"object_name_suffix"`
	Objects          []*object
	Imports          []string `json:"imports"`
	InterfaceName    string `json:"interface_name"`
}

var (
	ErrTooManyObjects = errors.New("too many objects")
)

var idCounter uint16
var usedIds sets.Set
var doc *document
var typeTmpl *template.Template
var mainBuf *bytes.Buffer

func parseFile(file string, w io.Writer) error {
	objects := []*object{}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	schema, err := ini.Load(data)
	if err != nil {
		return err
	}

	for _, section := range schema.Sections() {
		if section.Name() == "DEFAULT" {
			continue
		}
		fields := []*field{}

		for _, k := range section.Keys() {
			if k.Name() != "_id" {
				fields = append(fields, &field{
					Name:k.Name(),
					Type:k.Value(),
					CamelCase:fmt.Sprintf("%c%s", unicode.ToLower([]rune(k.Name())[0]), k.Name()[1:]),
				})
			}
		}

		name := section.Name()
		var id uint16

		if section.HasKey("_id") {
			id = uint16(section.Key("_id").MustUint())
			usedIds.Add(id)
		} else {
			id, err = getNextId()
			if err != nil {
				return err
			}
		}

		objects = append(objects, &object{
			Id:id,
			Name:name + doc.ObjectNameSuffix,
			RawName:name,
			Fields:fields,
		})
	}

	doc.Objects = append(doc.Objects, objects...)

	if err := typeTmpl.ExecuteTemplate(w, "objects", objects); err != nil {
		return err
	}

	return nil
}

func getNextId() (uint16, error) {
	for idCounter++; idCounter < (1 << 16) - 1; idCounter++ {
		if !usedIds.Contains(idCounter) {
			return idCounter, nil
		}
	}

	return 0, ErrTooManyObjects
}

func isVariableSize(o *object) bool {
	for _, f := range o.Fields {
		if f.Type == "string" {
			return true
		}
	}

	return false
}

func isArray(f *field) bool {
	return strings.HasPrefix(f.Type, "[")
}

func arraySize(f *field) int {
	t := f.Type
	n, err := strconv.ParseInt(t[1:strings.IndexByte(t, ']')], 10, 32)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	return int(n)
}

func arrayType(t string) string {
	return t[strings.LastIndexByte(t, ']') + 1:]
}

func executeTmpl(f *field, prefix string) (string, error) {
	buf := &bytes.Buffer{}
	var ft *template.Template
	var t string

	if isArray(f) {
		t = "array"
	} else {
		t = f.Type
	}

	ft = typeTmpl.Lookup(prefix + "_" + t)
	if ft == nil {
		return "", errors.New("template '" + prefix + "_" + t + "' not found")
	}

	err := ft.Execute(buf, f)
	return buf.String(), err
}

func write(f *field) (string, error) {
	return executeTmpl(f, "write")
}

func read(f *field) (string, error) {
	return executeTmpl(f, "read")
}

func writeArrayIndex(f *field) (string, error) {
	ai := typeTmpl.Lookup("array_index.tmpl")
	nf := &field{}
	nf.Type = arrayType(f.Type)
	buf := &bytes.Buffer{}
	err := ai.Execute(buf, f)
	if err != nil {
		return "", err
	}
	nf.Name = buf.String()
	return write(nf)
}

func readArrayIndex(f *field) (string, error) {
	ai := typeTmpl.Lookup("array_index.tmpl")
	nf := &field{}
	nf.Type = arrayType(f.Type)
	buf := &bytes.Buffer{}
	err := ai.Execute(buf, f)
	if err != nil {
		return "", err
	}
	nf.Name = buf.String()
	return read(nf)
}

var langFlag = flag.String("t", "", "target language")
var schemaFlag = flag.String("i", "", "input schema files pattern")
var outFlag = flag.String("o", "net_objects_result.go", "result file name")

func main() {
	flag.Parse()
	lang := *langFlag
	pattern := *schemaFlag

	if lang == "" {
		log.Fatalln("lang not set")
		return
	}

	if pattern == "" {
		log.Fatalln("schema files not set")
		return
	}

	files, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatalln(err)
		return
	}

	if len(files) == 0 {
		log.Println("no schema files found")
		return
	}

	typeTmpl = template.New("type").Funcs(template.FuncMap{
		"isVariableSize":isVariableSize,
		"isArray":isArray,
		"arraySize":arraySize,
		"write":write,
		"read":read,
		"writeArrayIndex":writeArrayIndex,
		"readArrayIndex":readArrayIndex,
	})

	for _, n := range bindata.AssetNames() {
		name := n[strings.IndexByte(n, '/') + 1:strings.LastIndexByte(n, '.')]
		newTmpl, err := typeTmpl.New(name).Parse(string(bindata.MustAsset(n)))
		if err != nil {
			log.Fatalln(err)
			return
		}
		typeTmpl, err = typeTmpl.AddParseTree(name, newTmpl.Tree)
		if err != nil {
			log.Fatalln(err)
			return
		}
	}

	cfgFile, err := ioutil.ReadFile("./bufobjects.json")
	if err != nil {
		log.Fatalln(err)
		return
	}
	doc = &document{
		Objects:[]*object{},
		PackageName:"main",
		InterfaceName:"Object",
		MaxObjectSize:4096,
	}
	if err := json.Unmarshal(cfgFile, doc); err != nil {
		log.Fatalln(err)
		return
	}

	mainBuf = &bytes.Buffer{}
	usedIds = hashset.New()
	for _, f := range files {
		if err = parseFile(f, mainBuf); err != nil {
			log.Fatalln(err)
			return
		}
	}

	docTmpl, err := template.New("doc").Parse(string(bindata.MustAsset(lang + "/doc.tmpl")))
	if err != nil {
		log.Fatalln(err)
		return
	}

	resFile, err := os.Create(*outFlag)
	if err != nil {
		log.Fatalln(err)
		return
	}
	doc.ObjectsImpl = mainBuf.String()
	err = docTmpl.ExecuteTemplate(resFile, "doc", doc)
	if err != nil {
		log.Fatalln(err)
	}
}
