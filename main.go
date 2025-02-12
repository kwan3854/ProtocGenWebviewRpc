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

// (1) go:embed 템플릿들
//
//go:embed templates/csharp_client.tmpl
var csharpClientTemplateStr string

//go:embed templates/csharp_server.tmpl
var csharpServerTemplateStr string

//go:embed templates/js_client.tmpl
var jsClientTemplateStr string

//go:embed templates/js_server.tmpl
var jsServerTemplateStr string

var (
	csharpClientTmpl *template.Template
	csharpServerTmpl *template.Template
	jsClientTmpl     *template.Template
	jsServerTmpl     *template.Template
)

func init() {
	csharpClientTmpl = template.Must(template.New("csharp_client").Parse(csharpClientTemplateStr))
	csharpServerTmpl = template.Must(template.New("csharp_server").Parse(csharpServerTemplateStr))
	jsClientTmpl = template.Must(template.New("js_client").Parse(jsClientTemplateStr))
	jsServerTmpl = template.Must(template.New("js_server").Parse(jsServerTemplateStr))
}

// -------------------- 구조체 & 메서드들 --------------------

type methodInfo struct {
	MethodName string
	InputType  string
	OutputType string
}

type serviceInfo struct {
	CsharpNamespace string
	ServiceName     string
	Methods         []methodInfo

	AllMessages   []string // e.g. ["HelloRequest","HelloResponse"]
	ProtoBaseName string   // e.g. "hello_service"
}

func main() {
	// 1) STDIN -> CodeGeneratorRequest
	reqBuf, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail("failed to read request: %v", err)
	}
	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(reqBuf, &req); err != nil {
		fail("failed to unmarshal CodeGeneratorRequest: %v", err)
	}

	// 2) 플러그인 파라미터 해석
	// ex) "--webviewrpc_out=cs_server,js_client:./out"
	//     => paramStr = "cs_server,js_client"
	paramStr := req.GetParameter()
	params := parseGeneratorParams(paramStr)
	genCSClient := (params["cs_client"] == "true")
	genCSServer := (params["cs_server"] == "true")
	genJSClient := (params["js_client"] == "true")
	genJSServer := (params["js_server"] == "true")

	// 3) Response 준비
	resp := &pluginpb.CodeGeneratorResponse{}

	// 4) .proto 파일들 순회
	for _, fileDesc := range req.ProtoFile {
		filename := fileDesc.GetName()

		// "file_to_generate"에 속한 .proto만 처리
		if !contains(req.FileToGenerate, filename) {
			continue
		}

		baseName := strings.TrimSuffix(filename, filepath.Ext(filename))

		// 파일 내 service들
		for _, svc := range fileDesc.GetService() {
			svcName := svc.GetName()

			// 메서드 목록
			var methods []methodInfo
			for _, m := range svc.GetMethod() {
				methods = append(methods, methodInfo{
					MethodName: m.GetName(),
					InputType:  shortTypeName(m.GetInputType()),
					OutputType: shortTypeName(m.GetOutputType()),
				})
			}

			svcData := serviceInfo{
				CsharpNamespace: getCsharpNamespace(fileDesc),
				ServiceName:     svcName,
				Methods:         methods,
				AllMessages:     collectAllMessages(fileDesc),
				ProtoBaseName:   baseName,
			}

			// ---------- C# Client ----------
			if genCSClient {
				outCode, e1 := renderTemplate(csharpClientTmpl, svcData)
				if e1 != nil {
					appendError(resp, e1.Error())
				} else {
					fileName := fmt.Sprintf("%s_%sClient.cs", baseName, svcName)
					resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
						Name:    &fileName,
						Content: &outCode,
					})
				}
			}

			// ---------- C# Server ----------
			if genCSServer {
				outCode, e2 := renderTemplate(csharpServerTmpl, svcData)
				if e2 != nil {
					appendError(resp, e2.Error())
				} else {
					fileName := fmt.Sprintf("%s_%sBase.cs", baseName, svcName)
					resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
						Name:    &fileName,
						Content: &outCode,
					})
				}
			}

			// ---------- JS Client ----------
			if genJSClient {
				outCode, e3 := renderTemplate(jsClientTmpl, svcData)
				if e3 != nil {
					appendError(resp, e3.Error())
				} else {
					fileName := fmt.Sprintf("%s_%sClient.js", baseName, svcName)
					resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
						Name:    &fileName,
						Content: &outCode,
					})
				}
			}

			// ---------- JS Server ----------
			if genJSServer {
				outCode, e4 := renderTemplate(jsServerTmpl, svcData)
				if e4 != nil {
					appendError(resp, e4.Error())
				} else {
					fileName := fmt.Sprintf("%s_%sBase.js", baseName, svcName)
					resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
						Name:    &fileName,
						Content: &outCode,
					})
				}
			}
		}
	}

	// 5) 응답 serialize -> STDOUT
	outBytes, err := proto.Marshal(resp)
	if err != nil {
		fail("failed to marshal response: %v", err)
	}
	os.Stdout.Write(outBytes)
}

// ------------- 도우미들 ------------------

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// parseGeneratorParams("cs_server,js_client") => map{"cs_server":"true","js_client":"true"}
func parseGeneratorParams(paramStr string) map[string]string {
	m := make(map[string]string)
	if paramStr == "" {
		return m
	}
	parts := strings.Split(paramStr, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = "true"
		}
	}
	return m
}

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// e.g. ".myapp.HelloRequest" -> "myappHelloRequest"
// 실제론 네임스페이스를 더 세밀하게 매핑해도 됨
func shortTypeName(full string) string {
	return strings.TrimPrefix(full, ".")
}

func collectAllMessages(fd *descriptorpb.FileDescriptorProto) []string {
	var out []string
	for _, md := range fd.GetMessageType() {
		out = append(out, md.GetName())
	}
	return out
}

func getCsharpNamespace(fd *descriptorpb.FileDescriptorProto) string {
	opts := fd.GetOptions()
	if opts != nil {
		_ = protojson.Format(opts.ProtoReflect().Interface())
	}
	if ns := fd.GetOptions().GetCsharpNamespace(); ns != "" {
		return ns
	}
	pkg := fd.GetPackage()
	if pkg == "" {
		return "DefaultNamespace"
	}
	return strings.Title(pkg)
}

func renderTemplate(tmpl *template.Template, data interface{}) (string, error) {
	var sb strings.Builder
	err := tmpl.Execute(&sb, data)
	if err != nil {
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
