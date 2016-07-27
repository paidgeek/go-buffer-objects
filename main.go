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
)

type field struct {
	Name      string
	CamelCase string
	Type      string
}

type object struct {
	Id      interface{}
	Name    string
	RawName string
	Fields  []*field
}

type document struct {
	ObjectsImpl      string
	ObjectNameSuffix string `json:"object_name_suffix"`
	ObjectIdType     string `json:"object_id_type"`
	Objects          []*object
	Imports          []string `json:"imports"`
	InterfaceName    string `json:"interface_name"`
	FactoryName      string `json:"factory_name"`
}

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
		if !section.HasKey("_id") {
			return errors.New("section '" + section.Name() + "' does not contain a _id")
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
		id := section.Key("_id").Value()

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

	ft = typeTmpl.Lookup("write_" + t)
	if ft == nil {
		return "", errors.New("template 'write_" + t + "' not found")
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

var langFlag = flag.String("lang", "", "target language")
var schemaFlag = flag.String("schema", "", "schema files pattern")
var outFlag = flag.String("out", "net_objects_result.go", "result file name")

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
	/*
		for k, v := range Templates {
			if strings.HasPrefix(k, lang) {
				typeTmpl, err = typeTmpl.Parse(v)
				if err != nil {
					log.Println(err)
					return
				}
			}
		}
		*/

	for _, n := range bindata.AssetNames() {
		name := n[strings.IndexByte(n, '/') + 1:strings.LastIndexByte(n, '.')]
		newTmpl, err := typeTmpl.New(name).Parse(string(bindata.MustAsset(n)))
		typeTmpl, err = typeTmpl.AddParseTree(name, newTmpl.Tree)
		if err != nil {
			log.Fatalln(err)
			return
		}
	}

	/*
		resFile, err := os.Create(*outFlag)
		if err != nil {
			return
		}*/
	cfgFile, err := ioutil.ReadFile("./bufobjects.json")
	if err != nil {
		log.Fatalln(err)
		return
	}
	doc = &document{
		Objects:[]*object{},
	}
	if err := json.Unmarshal(cfgFile, doc); err != nil {
		log.Fatalln(err)
		return
	}

	mainBuf = &bytes.Buffer{}
	if err = parseFile(files[0], mainBuf); err != nil {
		log.Fatalln(err)
		os.Remove(*outFlag)
		return
	}

	docTmpl, err := template.New("doc").Parse(string(bindata.MustAsset(lang + "/doc.tmpl")))
	if err != nil {
		log.Fatalln(err)
		return
	}

	doc.ObjectsImpl = mainBuf.String()
	err = docTmpl.ExecuteTemplate(os.Stdout, "doc", doc)

	/*
	for _, f := range files {
		if err = parseFile(f, resFile); err != nil {
			log.Fatalln(err)
			os.Remove(*outFlag)
			return
		}
	}*/
}
