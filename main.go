package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// (1) go:embed 로 템플릿 파일을 내장
//
//go:embed templates/csharp_client.tmpl
var csharpClientTemplateStr string

//go:embed templates/js_client.tmpl
var jsClientTemplateStr string

var csharpTmpl *template.Template
var jsTmpl *template.Template

func init() {
	// (2) 템플릿 파싱
	csharpTmpl = template.Must(template.New("csharp_client").
		Parse(csharpClientTemplateStr))
	jsTmpl = template.Must(template.New("js_client").
		Parse(jsClientTemplateStr))
}

type methodInfo struct {
	MethodName string
	InputType  string
	OutputType string
}

type serviceInfo struct {
	CsharpNamespace string
	ServiceName     string
	Methods         []methodInfo

	// JS 템플릿용
	AllMessages   []string // ex) ["HelloRequest", "HelloResponse"]
	ProtoBaseName string   // ex) "hello_service"
}

// main: 플러그인 진입점
func main() {
	// 1) stdin에서 CodeGeneratorRequest (바이너리) 읽기
	reqBuf, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read request: %v\n", err)
		os.Exit(1)
	}

	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(reqBuf, &req); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal CodeGeneratorRequest: %v\n", err)
		os.Exit(1)
	}

	// 2) CodeGeneratorResponse 준비
	resp := &pluginpb.CodeGeneratorResponse{}

	// 3) proto file들을 순회
	for _, fileDesc := range req.ProtoFile {
		filename := fileDesc.GetName()

		// "file_to_generate" 목록에 속한 .proto만 처리
		if !contains(req.FileToGenerate, filename) {
			continue
		}

		// fileDesc: *descriptorpb.FileDescriptorProto
		// services, messages, etc. 파싱
		baseName := strings.TrimSuffix(filename, filepath.Ext(filename)) // ex) "hello_service"

		for _, svc := range fileDesc.GetService() {
			svcName := svc.GetName() // "HelloService"

			// (A) 메서드 목록 수집
			var methods []methodInfo
			for _, m := range svc.GetMethod() {
				methods = append(methods, methodInfo{
					MethodName: m.GetName(),                      // e.g. "SayHello"
					InputType:  shortTypeName(m.GetInputType()),  // e.g. "HelloRequest"
					OutputType: shortTypeName(m.GetOutputType()), // e.g. "HelloResponse"
				})
			}

			svcData := serviceInfo{
				ServiceName: svcName,
				Methods:     methods,

				// (B) C# 네임스페이스 결정 (option csharp_namespace if set)
				CsharpNamespace: getCsharpNamespace(fileDesc),

				// (C) JS 템플릿용
				AllMessages:   collectAllMessages(fileDesc),
				ProtoBaseName: baseName,
			}

			// 4) C# 코드 생성
			csharpOut, err := renderTemplate(csharpTmpl, svcData)
			if err != nil {
				appendError(resp, err.Error())
				continue
			}

			// 5) JS 코드 생성
			jsOut, err := renderTemplate(jsTmpl, svcData)
			if err != nil {
				appendError(resp, err.Error())
				continue
			}

			// 6) 응답에 파일 추가 (C#, JS 각각)
			{
				file := &pluginpb.CodeGeneratorResponse_File{}
				fileName := fmt.Sprintf("%s_%sClient.cs", baseName, svcName)
				file.Name = &fileName
				file.Content = &csharpOut
				resp.File = append(resp.File, file)
			}

			{
				file := &pluginpb.CodeGeneratorResponse_File{}
				fileName := fmt.Sprintf("%s_%sClient.js", baseName, svcName)
				file.Name = &fileName
				file.Content = &jsOut
				resp.File = append(resp.File, file)
			}
		}
	}

	// 7) response 직렬화 후 stdout
	outBytes, err := proto.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal CodeGeneratorResponse: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(outBytes)
}

// ---------- 도우미 함수들 ----------

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// shortTypeName(".myapp.HelloRequest") -> "HelloRequest"
// gRPC fully qualified => C#/JS class 명으로 단순 치환
func shortTypeName(fullName string) string {
	return strings.TrimPrefix(fullName, ".")
}

// option csharp_namespace = "..."; 있으면 그걸 사용. 없으면 fallback
func getCsharpNamespace(fd *descriptorpb.FileDescriptorProto) string {
	opts := fd.GetOptions()
	if opts != nil {
		// protojson으로 살짝 파싱해서 csharp_namespace 찾는 트릭
		_, _ = protojson.Marshal(opts)
		// 예: {"csharp_namespace":"MyApp.Rpc","deprecated":false} 형태
		// 정석으론 descriptorpb.FileOptions.CsharpNamespace field를 봐도 됨.
		// or fd.GetOptions().GetCsharpNamespace()
	}

	if ns := fd.GetOptions().GetCsharpNamespace(); ns != "" {
		return ns
	}

	// fallback: package명을 C# 네임스페이스로
	pkg := fd.GetPackage()
	if pkg == "" {
		return "DefaultNamespace"
	}
	return strings.Title(pkg)
}

// messages 이름을 전부 수집(단순 예시)
func collectAllMessages(fd *descriptorpb.FileDescriptorProto) []string {
	var out []string
	for _, md := range fd.GetMessageType() {
		out = append(out, md.GetName())
	}
	return out
}

func renderTemplate(tmpl *template.Template, data interface{}) (string, error) {
	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func appendError(resp *pluginpb.CodeGeneratorResponse, msg string) {
	if resp.Error == nil {
		resp.Error = &msg
	} else {
		newErr := *resp.Error + "\n" + msg
		resp.Error = &newErr
	}
}
