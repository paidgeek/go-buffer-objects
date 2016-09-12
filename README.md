# Go Buffer Objects
[![Go Report Card](https://goreportcard.com/badge/github.com/paidgeek/go-buffer-objects)](https://goreportcard.com/report/github.com/paidgeek/go-buffer-objects)
[![codebeat badge](https://codebeat.co/badges/6b088eff-b986-4848-aaae-5e341432b05a)](https://codebeat.co/projects/github-com-paidgeek-go-buffer-objects)

Generate code for fast serialization and deserialization based on YAML schema.

## Installation
```
go get github.com/paidgeek/go-buffer-objects
go install github.com/paidgeek/go-buffer-objects
```

## Usage
```
Usage of go-buffer-objects:

    $ go-buffer-objects [options]

Options:
    -i string
        schema files pattern
    -interface string
        interface name (default "BufObject")
    -max-size uint
        max object size (used as read/write buffer size) (default 4096)
    -name-suffix string
        optional object name suffix
    -o string
        result file path (default "bufobjects_gen.go")
    -p string
        result package name (default "main")
    -t string
        target language
```

## Example
Given the following schema:
```yaml
Hello:
   Text: "string"
   Time: "int64"
```
Command: `$ bufobjects -t go -i schema.yaml -o message/gen.go -interface Message -p message -name-suffix Message`
will generate a struct implementing the following interface:
```go
type Message interface {
	Id() uint16
	Size() int
	IsVariableSize() bool
	MarshalBody(buf []byte, off int) int
	UnmarshalBody(buf []byte, off int) int
	Reset()
}
```
and helper functions:
```go
func NewHelloMessage(text  string,time  int64) *HelloMessage {}
func NewMessageWithId(id uint16) Message {}
func WriteMessageAt(o Message, buf []byte) (n int) {}
func WriteMessageTo(o Message, buf []byte, w io.Writer) (n int, err error) {}
func ReadMessageAt(buf []byte) (o Message) {}
func ReadMessageFrom(buf []byte, r io.Reader) (o Message, err error) {}
```
Using `WriteMessage*` you can serialize any generated struct and deserialize it using `ReadMessage*`.
```go
buf := make([]byte, message.MaxSize)
msg := message.NewHelloMessage("Hello, World!", time.Now().Unix())
message.WriteMessageAt(msg, buf)

res := message.ReadMessageAt(buf)
switch res.Id() {
case message.IdHello:
    resMsg := res.(*message.HelloMessage)

    fmt.Printf("%s: %s", time.Unix(resMsg.Time, 0), resMsg.Text)
}
```
Object's id and size are serialized along with data so `ReadMessage*` knows how much to read and what struct to return.

## Benchmark
Benchmark with: [github.com/alecthomas/go_serialization_benchmarks](https://github.com/alecthomas/go_serialization_benchmarks).
<pre>
<b>BenchmarkBufobjectsMarshal-8</b>            10000000               124 ns/op               0 B/op          0 allocs/op
<b>BenchmarkBufobjectsUnmarshal-8</b>           5000000               301 ns/op              96 B/op          3 allocs/op
BenchmarkMsgpMarshal-8                   5000000               373 ns/op             128 B/op          1 allocs/op
BenchmarkMsgpUnmarshal-8                 2000000               658 ns/op             112 B/op          3 allocs/op
BenchmarkVmihailencoMsgpackMarshal-8      500000              2732 ns/op             368 B/op          6 allocs/op
BenchmarkVmihailencoMsgpackUnmarshal-8    500000              3002 ns/op             352 B/op         13 allocs/op
BenchmarkJsonMarshal-8                    200000              6815 ns/op            1232 B/op         10 allocs/op
BenchmarkJsonUnmarshal-8                  200000              6293 ns/op             416 B/op          7 allocs/op
BenchmarkEasyJsonMarshal-8                500000              3179 ns/op             784 B/op          5 allocs/op
BenchmarkEasyJsonUnmarshal-8              500000              2533 ns/op             160 B/op          4 allocs/op
BenchmarkBsonMarshal-8                   1000000              2498 ns/op             392 B/op         10 allocs/op
BenchmarkBsonUnmarshal-8                  500000              3163 ns/op             248 B/op         21 allocs/op
BenchmarkGobMarshal-8                    1000000              1931 ns/op              48 B/op          2 allocs/op
BenchmarkGobUnmarshal-8                  1000000              1982 ns/op             112 B/op          3 allocs/op
BenchmarkXdrMarshal-8                     500000              3339 ns/op             425 B/op         20 allocs/op
BenchmarkXdrUnmarshal-8                   500000              2931 ns/op             232 B/op         11 allocs/op
BenchmarkUgorjiCodecMsgpackMarshal-8      200000              6363 ns/op            2753 B/op          8 allocs/op
BenchmarkUgorjiCodecMsgpackUnmarshal-8    200000              6122 ns/op            3008 B/op          6 allocs/op
BenchmarkUgorjiCodecBincMarshal-8         300000              5933 ns/op            2785 B/op          8 allocs/op
BenchmarkUgorjiCodecBincUnmarshal-8       200000              6507 ns/op            3168 B/op          9 allocs/op
BenchmarkSerealMarshal-8                  200000              6867 ns/op             912 B/op         21 allocs/op
BenchmarkSerealUnmarshal-8                200000              6133 ns/op            1008 B/op         34 allocs/op
BenchmarkBinaryMarshal-8                 1000000              2321 ns/op             256 B/op         16 allocs/op
BenchmarkBinaryUnmarshal-8                500000              2713 ns/op             336 B/op         22 allocs/op
BenchmarkFlatBuffersMarshal-8            3000000               505 ns/op               0 B/op          0 allocs/op
BenchmarkFlatBuffersUnmarshal-8          3000000               470 ns/op             112 B/op          3 allocs/op
BenchmarkCapNProtoMarshal-8              2000000               661 ns/op              56 B/op          2 allocs/op
BenchmarkCapNProtoUnmarshal-8            2000000               725 ns/op             200 B/op          6 allocs/op
BenchmarkCapNProto2Marshal-8             1000000              1842 ns/op             244 B/op          3 allocs/op
BenchmarkCapNProto2Unmarshal-8           1000000              1838 ns/op             320 B/op          6 allocs/op
BenchmarkHproseMarshal-8                 1000000              1684 ns/op             479 B/op          8 allocs/op
BenchmarkHproseUnmarshal-8               1000000              2008 ns/op             320 B/op         10 allocs/op
BenchmarkProtobufMarshal-8               1000000              1748 ns/op             200 B/op          7 allocs/op
BenchmarkProtobufUnmarshal-8             1000000              1356 ns/op             192 B/op         10 allocs/op
BenchmarkGoprotobufMarshal-8             1000000              1055 ns/op             312 B/op          4 allocs/op
BenchmarkGoprotobufUnmarshal-8           1000000              1467 ns/op             432 B/op          9 allocs/op
BenchmarkGogoprotobufMarshal-8           5000000               250 ns/op              64 B/op          1 allocs/op
BenchmarkGogoprotobufUnmarshal-8         3000000               403 ns/op              96 B/op          3 allocs/op
BenchmarkColferMarshal-8                10000000               227 ns/op              64 B/op          1 allocs/op
BenchmarkColferUnmarshal-8               5000000               331 ns/op             112 B/op          3 allocs/op
BenchmarkGencodeMarshal-8                5000000               311 ns/op              80 B/op          2 allocs/op
BenchmarkGencodeUnmarshal-8              5000000               373 ns/op             112 B/op          3 allocs/op
BenchmarkGencodeUnsafeMarshal-8         10000000               179 ns/op              48 B/op          1 allocs/op
BenchmarkGencodeUnsafeUnmarshal-8        5000000               277 ns/op              96 B/op          3 allocs/op
BenchmarkXDR2Marshal-8                   5000000               306 ns/op              64 B/op          1 allocs/op
BenchmarkXDR2Unmarshal-8                 5000000               265 ns/op              32 B/op          2 allocs/op
</pre>
