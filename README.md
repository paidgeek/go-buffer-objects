# bufobjects
Generate code for fast serialization and deserialization based on YAML schema. Currently only for Go.

## Getting Started
```
go get github.com/paidgeek/bufobjects
go install github.com/paidgeek/bufobjects
```

## Usage
```
Usage of bufobjects:

    $ bufobjects [options]

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
<b>BenchmarkBufobjectsUnmarshal-8</b>           3000000               434 ns/op             128 B/op          5 allocs/op
BenchmarkMsgpMarshal-8                   5000000               340 ns/op             128 B/op          1 allocs/op
BenchmarkMsgpUnmarshal-8                 2000000               629 ns/op             112 B/op          3 allocs/op
BenchmarkVmihailencoMsgpackMarshal-8      500000              2721 ns/op             368 B/op          6 allocs/op
BenchmarkVmihailencoMsgpackUnmarshal-8    500000              3050 ns/op             352 B/op         13 allocs/op
BenchmarkJsonMarshal-8                    200000              6289 ns/op            1232 B/op         10 allocs/op
BenchmarkJsonUnmarshal-8                  200000              6468 ns/op             416 B/op          7 allocs/op
BenchmarkEasyJsonMarshal-8                500000              2907 ns/op             784 B/op          5 allocs/op
BenchmarkEasyJsonUnmarshal-8              500000              2667 ns/op             160 B/op          4 allocs/op
BenchmarkBsonMarshal-8                   1000000              2481 ns/op             392 B/op         10 allocs/op
BenchmarkBsonUnmarshal-8                  500000              3228 ns/op             248 B/op         21 allocs/op
BenchmarkGobMarshal-8                    1000000              2021 ns/op              48 B/op          2 allocs/op
BenchmarkGobUnmarshal-8                  1000000              2019 ns/op             112 B/op          3 allocs/op
BenchmarkXdrMarshal-8                     500000              3339 ns/op             456 B/op         21 allocs/op
BenchmarkXdrUnmarshal-8                   500000              2727 ns/op             237 B/op         11 allocs/op
BenchmarkUgorjiCodecMsgpackMarshal-8      300000              5407 ns/op            2753 B/op          8 allocs/op
BenchmarkUgorjiCodecMsgpackUnmarshal-8    300000              5780 ns/op            3008 B/op          6 allocs/op
BenchmarkUgorjiCodecBincMarshal-8         300000              5351 ns/op            2785 B/op          8 allocs/op
BenchmarkUgorjiCodecBincUnmarshal-8       200000              6783 ns/op            3168 B/op          9 allocs/op
BenchmarkSerealMarshal-8                  200000              7253 ns/op             912 B/op         21 allocs/op
BenchmarkSerealUnmarshal-8                200000              6444 ns/op            1008 B/op         34 allocs/op
BenchmarkBinaryMarshal-8                 1000000              2579 ns/op             256 B/op         16 allocs/op
BenchmarkBinaryUnmarshal-8                500000              2762 ns/op             336 B/op         22 allocs/op
BenchmarkFlatBuffersMarshal-8            3000000               516 ns/op               0 B/op          0 allocs/op
BenchmarkFlatBuffersUnmarshal-8          3000000               450 ns/op             112 B/op          3 allocs/op
BenchmarkCapNProtoMarshal-8              2000000               682 ns/op              56 B/op          2 allocs/op
BenchmarkCapNProtoUnmarshal-8            2000000               703 ns/op             200 B/op          6 allocs/op
BenchmarkCapNProto2Marshal-8             1000000              1832 ns/op             244 B/op          3 allocs/op
BenchmarkCapNProto2Unmarshal-8           1000000              1874 ns/op             320 B/op          6 allocs/op
BenchmarkHproseMarshal-8                 1000000              1652 ns/op             473 B/op          8 allocs/op
BenchmarkHproseUnmarshal-8               1000000              1991 ns/op             320 B/op         10 allocs/op
BenchmarkProtobufMarshal-8               1000000              1738 ns/op             200 B/op          7 allocs/op
BenchmarkProtobufUnmarshal-8             1000000              1336 ns/op             192 B/op         10 allocs/op
BenchmarkGoprotobufMarshal-8             2000000               976 ns/op             312 B/op          4 allocs/op
BenchmarkGoprotobufUnmarshal-8           1000000              1378 ns/op             432 B/op          9 allocs/op
BenchmarkGogoprotobufMarshal-8           5000000               253 ns/op              64 B/op          1 allocs/op
BenchmarkGogoprotobufUnmarshal-8         5000000               397 ns/op              96 B/op          3 allocs/op
BenchmarkColferMarshal-8                10000000               221 ns/op              64 B/op          1 allocs/op
BenchmarkColferUnmarshal-8               5000000               332 ns/op             112 B/op          3 allocs/op
BenchmarkGencodeMarshal-8                5000000               304 ns/op              80 B/op          2 allocs/op
BenchmarkGencodeUnmarshal-8              5000000               359 ns/op             112 B/op          3 allocs/op
BenchmarkGencodeUnsafeMarshal-8         10000000               170 ns/op              48 B/op          1 allocs/op
BenchmarkGencodeUnsafeUnmarshal-8        5000000               268 ns/op              96 B/op          3 allocs/op
BenchmarkXDR2Marshal-8                   5000000               284 ns/op              64 B/op          1 allocs/op
BenchmarkXDR2Unmarshal-8                 5000000               264 ns/op              32 B/op          2 allocs/op
</pre>
**Note**: Deserializing allocates memory for strings so the source buffer can be re-used (unlike Gogoprotobuf or Gencode). That's why unmarshal time is noticeably slower.
 
