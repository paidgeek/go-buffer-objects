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
	yamlData := make(map[string]interface{})
	err = yaml.Unmarshal(data, yamlData)
	if err != nil {
		return err
	}

	for key, val := range yamlData {
		fields := []*field{}
		var id uint16

		if reflect.ValueOf(val).Kind() == reflect.Map {
			fieldData := val.(map[interface{}]interface{})

			for fn, ft := range fieldData {
				if fn == "_id" {
					continue
				}

				fieldName := fmt.Sprintf("%v", fn)
				fieldType := fmt.Sprintf("%v", ft)
				fields = append(fields, &field{
					Name:fieldName,
					Type:fieldType,
					CamelCase:fmt.Sprintf("%c%s", unicode.ToLower([]rune(fieldName)[0]), fieldName[1:]),
				})
			}

			if idVal, ok := fieldData["_id"]; ok {
				id = idVal.(uint16)
				usedIds.Add(id)
			} else {
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

		objects = append(objects, &object{
			Id:id,
			Name:key + doc.ObjectNameSuffix,
			RawName:key,
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

var schemaFlag = flag.String("i", "", "input schema files pattern")
var outFlag = flag.String("o", "bufobjects_gen.go", "result file name")
var langFlag = flag.String("lang", "", "target language")
var pkgFlag = flag.String("pkg", "main", "result package name")
var interfaceNameFlag = flag.String("interface", "Object", "interface name")
var suffixFlag = flag.String("name-suffix", "", "object name suffix")
var maxSizeFlag = flag.Uint("max-size", 4096, "max object size")

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

	doc = &document{
		Objects:[]*object{},
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
