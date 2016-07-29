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
	"github.com/emirpasic/gods/sets"
	"github.com/emirpasic/gods/sets/hashset"
	"gopkg.in/yaml.v2"
	"reflect"
	"regexp"
)

type Field struct {
	Name      string
	CamelCase string
	Type      string
	ArraySize int
	IsObject  bool
	IsArray   bool
	IsSlice   bool
}

type Object struct {
	Id             uint16
	Name           string
	RawName        string
	IsVariableSize bool
	Fields         []*Field
}

type Document struct {
	MaxObjectSize    int `json:"max_object_size"`
	PackageName      string `json:"package_name"`
	ObjectsImpl      string
	ObjectNameSuffix string `json:"object_name_suffix"`
	Objects          []*Object
	Imports          []string `json:"imports"`
	InterfaceName    string `json:"interface_name"`
}

var (
	ErrTooManyObjects = errors.New("too many objects")
)

var idCounter uint16
var usedIds sets.Set
var doc *Document
var typeTmpl *template.Template
var mainBuf *bytes.Buffer

func parseFile(file string, w io.Writer) error {
	objects := []*Object{}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	yamlData := &yaml.MapSlice{}
	err = yaml.Unmarshal(data, yamlData)
	if err != nil {
		return err
	}

	for _, val := range *yamlData {
		fields := []*Field{}
		id := uint16(0)
		key := val.Key.(string)

		if reflect.ValueOf(val.Value).Kind() == reflect.Slice {
			for _, fieldData := range val.Value.(yaml.MapSlice) {
				fieldName := fmt.Sprintf("%v", fieldData.Key)
				fieldType := fmt.Sprintf("%v", fieldData.Value)

				if fieldName == "_id" {
					parsedId, err := strconv.ParseUint(fieldType, 10, 16)
					if err != nil {
						return err
					}
					id = uint16(parsedId)
					usedIds.Add(id)

					continue
				}

				f := &Field{
					Name:fieldName,
					Type:fieldType,
					CamelCase:fmt.Sprintf("%c%s", unicode.ToLower([]rune(fieldName)[0]), fieldName[1:]),
				}
				f.IsObject = isObject(f)
				f.IsSlice = isSlice(f)
				f.IsArray = isArray(f)
				if f.IsArray {
					f.ArraySize = arraySize(f)
				}
				f.Type = baseType(f)
				fields = append(fields, f)
			}

			if id == 0 {
				id, err = getNextId()
				if err != nil {
					return err
				}
			}
		} else {
			id, err = getNextId()
			if err != nil {
				return err
			}
		}

		obj := &Object{
			Id:id,
			Name:key + doc.ObjectNameSuffix,
			RawName:key,
			Fields:fields,
		}
		objects = append(objects, obj)
	}

	doc.Objects = append(doc.Objects, objects...)
	for _, obj := range doc.Objects {
		obj.IsVariableSize = isVariableSize(obj)
	}

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

func getObjectForType(t string) *Object {
	for _, obj := range doc.Objects {
		if obj.RawName == t {
			return obj
		}
	}

	return nil
}

func executeTmpl(name string, in interface{}) (string, error) {
	buf := &bytes.Buffer{}
	var ft *template.Template
	ft = typeTmpl.Lookup(name)
	if ft == nil {
		return "", errors.New("template '" + name + "' not found")
	}

	err := ft.Execute(buf, in)
	return buf.String(), err
}

func write(f *Field) (string, error) {
	var t string

	if f.IsObject && (f.IsArray || f.IsSlice) {
		t = "object_indexed"
	} else if isObject(f) {
		t = "object"
	} else if isArray(f) {
		t = "array"
	} else if isSlice(f) {
		t = "slice"
	} else {
		t = f.Type
	}

	return executeTmpl("write/write_" + t, f)
}

func read(f *Field) (string, error) {
	var t string

	if f.IsObject && (f.IsArray || f.IsSlice) {
		t = "object_indexed"
	} else if isObject(f) {
		t = "object"
	} else if isArray(f) {
		t = "array"
	} else if isSlice(f) {
		t = "slice"
	} else {
		t = f.Type
	}

	return executeTmpl("read/read_" + t, f)
}

func writeArrayIndex(f *Field) (string, error) {
	ai := typeTmpl.Lookup("array_index")
	nf := &Field{}
	nf.Type = arrayType(f.Type)
	buf := &bytes.Buffer{}
	err := ai.Execute(buf, f)
	if err != nil {
		return "", err
	}
	nf.Name = buf.String()
	return write(nf)
}

func readArrayIndex(f *Field) (string, error) {
	ai := typeTmpl.Lookup("array_index")
	nf := &Field{}
	nf.Type = arrayType(f.Type)
	buf := &bytes.Buffer{}
	err := ai.Execute(buf, f)
	if err != nil {
		return "", err
	}
	nf.Name = buf.String()
	return read(nf)
}

var schemaFlag = flag.String("i", "", "schema files pattern")
var outFlag = flag.String("o", "bufobjects_gen.go", "result file path")
var langFlag = flag.String("t", "", "target language")
var pkgFlag = flag.String("p", "main", "result package name")
var interfaceNameFlag = flag.String("interface", "BufObject", "interface name")
var suffixFlag = flag.String("name-suffix", "", "optional object name suffix")
var maxSizeFlag = flag.Uint("max-size", 4096, "max object size (used as read/write buffer size)")

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
		"write":write,
		"read":read,
		"writeArrayIndex":writeArrayIndex,
		"readArrayIndex":readArrayIndex,
		"baseSizeOf":baseSizeOf,
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

	doc = &Document{
		Objects:[]*Object{},
		PackageName:*pkgFlag,
		InterfaceName:*interfaceNameFlag,
		ObjectNameSuffix:*suffixFlag,
		MaxObjectSize:int(*maxSizeFlag),
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
	if lang == "go" {
		for _, obj := range doc.Objects {
			for _, f := range obj.Fields {
				if strings.Contains(f.Type, "float") {
					doc.Imports = append(doc.Imports, "unsafe")
					goto OUT
				}
			}
		}
	}
	OUT:

	err = docTmpl.ExecuteTemplate(resFile, "doc", doc)
	if err != nil {
		log.Fatalln(err)
	}
}

// utils

func baseType(f *Field) string {
	idx := strings.LastIndexByte(f.Type, ']')
	t := f.Type
	if idx > -1 {
		t = f.Type[idx + 1:]
	}
	return t
}

func isVariableSize(o *Object) bool {
	for _, f := range o.Fields {
		if f.Type == "string" || isSlice(f) {
			return true
		} else if isObject(f) {
			obj := getObjectForType(f.Type)
			if obj == nil {
				log.Fatalf("%v not defined\n", f.Type)
				os.Exit(1)
			}
			if isVariableSize(obj) {
				return true
			}
		}
	}

	return false
}

func baseSizeOf(f *Field) int {
	var t = f.Type

	if isArray(f) || isSlice(f) {
		t = arrayType(t)
	}

	switch t {
	case "bool":
		return 1
	case "byte":
		return 1
	case "int":
		return 4
	case "int8":
		return 1
	case "int16":
		return 2
	case "int32":
		return 4
	case "int64":
		return 8
	case "uint":
		return 4
	case "uint8":
		return 1
	case "uint16":
		return 2
	case "uint32":
		return 4
	case "uint64":
		return 8
	case "float32":
		return 4
	case "float64":
		return 8
	}

	log.Fatalf("baseSizeOf: invalid type %v\n", t)
	os.Exit(1)
	return 0
}

func isArray(f *Field) bool {
	matched, err := regexp.MatchString("^\\[[0-9]", f.Type)
	if err != nil {
		log.Fatalln(err)
		os.Exit(1)
	}
	return matched
}

func isObject(f *Field) bool {
	return isObjectType(f.Type)
}

func isObjectType(t string) bool {
	idx := strings.LastIndexByte(t, ']')
	if idx > -1 {
		t = t[idx + 1:]
	}
	return unicode.IsUpper([]rune(t)[0])
}

func isSlice(f *Field) bool {
	return strings.HasPrefix(f.Type, "[]")
}

func arraySize(f *Field) int {
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
