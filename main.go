package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// (1) Embed templates

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

// -------------------- Struct & Methods --------------------

type methodInfo struct {
	MethodName string
	InputType  string
	OutputType string
}

type serviceInfo struct {
	CsharpNamespace string
	ServiceName     string
	Methods         []methodInfo

	AllMessages   []string
	ProtoBaseName string
}

func main() {
	// 1) STDIN -> CodeGeneratorRequest
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail("failed to read request: %v", err)
	}
	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		fail("failed to unmarshal CodeGeneratorRequest: %v", err)
	}

	// 2) parse param (e.g. "cs_server,cs_client,js_server,js_client")
	paramStr := req.GetParameter()
	params := parseGeneratorParams(paramStr)
	genCSClient := (params["cs_client"] == "true")
	genCSServer := (params["cs_server"] == "true")
	genJSClient := (params["js_client"] == "true")
	genJSServer := (params["js_server"] == "true")

	resp := &pluginpb.CodeGeneratorResponse{}

	// 3) .proto file -> .cs, .js file
	for _, fd := range req.ProtoFile {
		filename := fd.GetName()
		if !contains(req.FileToGenerate, filename) {
			continue
		}
		baseName := strings.TrimSuffix(filename, filepath.Ext(filename))

		// collect service info
		for _, svc := range fd.GetService() {
			svcName := svc.GetName()

			// collect method info
			var methods []methodInfo
			for _, m := range svc.GetMethod() {
				methods = append(methods, methodInfo{
					MethodName: m.GetName(),
					InputType:  shortTypeName(m.GetInputType()),
					OutputType: shortTypeName(m.GetOutputType()),
				})
			}

			svcData := serviceInfo{
				CsharpNamespace: getCsharpNamespace(fd),
				ServiceName:     svcName,
				Methods:         methods,
				AllMessages:     collectAllMessages(fd),
				ProtoBaseName:   baseName,
			}

			// (A) C# Client
			if genCSClient {
				out, e := renderTemplate(csharpClientTmpl, svcData)
				if e != nil {
					appendError(resp, e.Error())
				} else {
					fileName := fmt.Sprintf("%s_%sClient.cs", baseName, svcName)
					resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
						Name:    &fileName,
						Content: &out,
					})
				}
			}

			// (B) C# Server
			if genCSServer {
				out, e := renderTemplate(csharpServerTmpl, svcData)
				if e != nil {
					appendError(resp, e.Error())
				} else {
					fileName := fmt.Sprintf("%s_%sBase.cs", baseName, svcName)
					resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
						Name:    &fileName,
						Content: &out,
					})
				}
			}

			// (C) JS Client
			if genJSClient {
				out, e := renderTemplate(jsClientTmpl, svcData)
				if e != nil {
					appendError(resp, e.Error())
				} else {
					fileName := fmt.Sprintf("%s_%sClient.js", baseName, svcName)
					resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
						Name:    &fileName,
						Content: &out,
					})
				}
			}

			// (D) JS Server
			if genJSServer {
				out, e := renderTemplate(jsServerTmpl, svcData)
				if e != nil {
					appendError(resp, e.Error())
				} else {
					fileName := fmt.Sprintf("%s_%sBase.js", baseName, svcName)
					resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
						Name:    &fileName,
						Content: &out,
					})
				}
			}
		}
	}

	// 4) serialize response -> stdout
	outBytes, err := proto.Marshal(resp)
	if err != nil {
		fail("failed to marshal CodeGeneratorResponse: %v", err)
	}
	os.Stdout.Write(outBytes)
}

// ---------- Helper -----------

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

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

func shortTypeName(full string) string {
	// e.g. ".helloworld.HelloRequest" -> "helloworld.HelloRequest"
	s := strings.TrimPrefix(full, ".")
	// split
	parts := strings.Split(s, ".")
	// take last
	return parts[len(parts)-1]
}

func getCsharpNamespace(fd *descriptorpb.FileDescriptorProto) string {
	if ns := fd.GetOptions().GetCsharpNamespace(); ns != "" {
		return ns
	}
	pkg := fd.GetPackage()
	if pkg == "" {
		return "DefaultNamespace"
	}
	return strings.Title(pkg)
}

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
