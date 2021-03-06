package gengocli

import (
	"bytes"
	"sort"
	"strings"
	"text/template"
	"unicode"

	pbdescriptor "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/descriptor"
	strcase2 "github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	strcase "github.com/stoewer/go-strcase"
)

func removePackage(s string) string {
	sp := strings.Split(s, ".")
	return sp[len(sp)-1]
}

// fixAcronyms fixes strings with acronyms:
// For example,
//   "Id" => "ID"
//   "Gid" => "GID"
//   "Pkid" => "PKID"
//   "GroupId" => "GroupID"
//   "Url" => "URL"
//   "Uri" => "URI"
func fixAcronyms(s string) string {
	if strings.HasSuffix(s, "Id") {
		if len(s) <= 4 {
			return strings.ToUpper(s)
		}
		return s[:len(s)-2] + "ID"
	}
	if strings.HasSuffix(s, "id") && len(s) <= 4 {
		return strings.ToUpper(s)
	}
	if s == "Url" {
		return "URL"
	}
	if s == "Uri" {
		return "URI"
	}
	return s
}

// Lowers first uppercase characters (Foo => foo, FOOBar => fooBar)
func lowerPrefix(s string) (lower string) {
	for pos, char := range s {
		if unicode.IsUpper(char) {
			lower = lower + string(unicode.ToLower(char))
		} else {
			if pos > 1 {
				lower = lower[:len(lower)-1] + s[pos-1:]
			} else {
				lower = lower + s[pos:]
			}
			break
		}
	}
	return
}

func (cfg GeneratorOptions) fieldToType(pkg string, f *descriptor.Field, reg *descriptor.Registry) (string, string) {
	name := *f.Name
	dashName := strcase.KebabCase(name)
	ucName := fixAcronyms(strcase2.ToCamel(name))
	switch f.GetType() {
	case pbdescriptor.FieldDescriptorProto_TYPE_DOUBLE:
		fallthrough
	case pbdescriptor.FieldDescriptorProto_TYPE_FLOAT:
		flag := `cli.Float64Flag{Name: "` + dashName + `"},`
		req := ucName + `: c.Float64("` + dashName + `"),`
		return flag, req
	case pbdescriptor.FieldDescriptorProto_TYPE_UINT64:
		flag := `cli.Uint64Flag{Name: "` + dashName + `"},`
		req := ucName + `: c.Uint64("` + dashName + `"),`
		return flag, req

	case pbdescriptor.FieldDescriptorProto_TYPE_UINT32:
		flag := `cli.UintFlag{Name: "` + dashName + `"},`
		req := ucName + `: c.Uint("` + dashName + `"),`
		return flag, req

	case pbdescriptor.FieldDescriptorProto_TYPE_INT32:
		fallthrough
	case pbdescriptor.FieldDescriptorProto_TYPE_FIXED32:
		fallthrough
	case pbdescriptor.FieldDescriptorProto_TYPE_SFIXED32:
		fallthrough
	case pbdescriptor.FieldDescriptorProto_TYPE_SINT32:
		flag := `cli.IntFlag{Name: "` + dashName + `"},`
		req := ucName + `: int32(c.Int("` + dashName + `")),`
		return flag, req

	case pbdescriptor.FieldDescriptorProto_TYPE_FIXED64:
		fallthrough
	case pbdescriptor.FieldDescriptorProto_TYPE_SFIXED64:
		fallthrough
	case pbdescriptor.FieldDescriptorProto_TYPE_SINT64:
		fallthrough
	case pbdescriptor.FieldDescriptorProto_TYPE_INT64:
		flag := `cli.Int64Flag{Name: "` + dashName + `"},`
		req := ucName + `: c.Int64("` + dashName + `"),`
		return flag, req
	case pbdescriptor.FieldDescriptorProto_TYPE_BOOL:
		flag := `cli.BoolFlag{Name: "` + dashName + `"},`
		req := ucName + `: c.Bool("` + dashName + `"),`
		return flag, req
	case pbdescriptor.FieldDescriptorProto_TYPE_STRING:
		var flag string
		var req string
		if f.GetLabel() == pbdescriptor.FieldDescriptorProto_LABEL_REPEATED {
			flag = `cli.StringSliceFlag{Name: "` + dashName + `"},`
			req = ucName + `: c.StringSlice("` + dashName + `"),`
		} else {
			flag = `cli.StringFlag{Name: "` + dashName + `"},`
			req = ucName + `: c.String("` + dashName + `"),`
		}
		return flag, req
	case pbdescriptor.FieldDescriptorProto_TYPE_GROUP:
		return "", ""
	case pbdescriptor.FieldDescriptorProto_TYPE_MESSAGE:
		return "", ""
	case pbdescriptor.FieldDescriptorProto_TYPE_BYTES:
		return "", ""
	case pbdescriptor.FieldDescriptorProto_TYPE_ENUM:
		return "", ""
	}
	return "", ""
}

func (cfg GeneratorOptions) methodToCommand(name string, m *descriptor.Method, reg *descriptor.Registry) (string, error) {
	methodName := *m.Name
	methodNameDashed := strcase.KebabCase(*m.Name)
	requestType := removePackage(*m.InputType)
	// responseType := removePackage(*m.OutputType)
	// prefix := strings.ToUpper(name)
	// actionName := prefix + "_" + strings.ToUpper(strcase.SnakeCase(methodName))

	msg, merr := reg.LookupMsg("", *m.InputType)
	if merr != nil {
		return "", merr
	}

	flags := []string{}
	reqs := []string{}
	for _, f := range msg.Fields {
		flag, req := cfg.fieldToType(msg.File.GetPackage(), f, reg)
		if flag != "" {
			flags = append(flags, flag)
		}
		if req != "" {
			reqs = append(reqs, req)
		}
	}

	flagsStr := "[]cli.Flag{},"
	if len(flags) != 0 {
		flagsStr = `[]cli.Flag{` + "\n\t\t\t\t" + strings.Join(flags, "\n\t\t\t\t") + "\n\t\t\t},"
	}

	reqStr := `&` + requestType + `{}`
	if len(reqs) != 0 {
		reqStr = `&` + requestType + `{
					` + strings.Join(reqs, "\n\t\t\t\t\t") + `
				}`
	}

	return "\n\t\t" + `cli.Command{
			Name: "` + methodNameDashed + `",
			Flags: ` + flagsStr + `
			Action: func(c *cli.Context) error {
				req := ` + reqStr + `
				resp, err := clientFn().` + methodName + `(context.TODO(), req)
				if err != nil {
					return err
				}
				s, marshalErr := json.MarshalIndent(resp, "", "  ")
				if marshalErr != nil {
					return marshalErr
				}
				fmt.Printf("%s\n", s)
				return nil
			},
		},`, nil
}

func (cfg GeneratorOptions) serviceToCLI(s *descriptor.Service, reg *descriptor.Registry) (string, error) {
	commands := []string{}
	methods := s.Methods
	sort.Slice(methods, func(i, j int) bool { return *methods[i].Name < *methods[j].Name })
	for _, m := range methods {
		cmds, err := cfg.methodToCommand(*s.Name, m, reg)
		if err != nil {
			return "", err
		}
		commands = append(commands, cmds)
	}
	return strings.Join(commands, ""), nil
}

func generateGoCLI(file *descriptor.File, registry *descriptor.Registry, options GeneratorOptions) (string, error) {
	f, err := registry.LookupFile(file.GetName())
	if err != nil {
		return "", err
	}

	commands := []string{}
	if len(f.Services) != 1 {
		return "", errors.Errorf("cli generator only supports a single service definition")
	}
	service := f.Services[0]
	serviceName := *service.Name + "Client"
	cmds, cerr := options.serviceToCLI(service, registry)
	if cerr != nil {
		return "", cerr
	}
	commands = append(commands, cmds)

	buf := new(bytes.Buffer)
	tmpl, err := template.New("").Parse(`// Code initially generated by protoc-gen-gocli
// InputID: {{.InputID}}

package proto

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/urfave/cli"
)

// Commands are autogenerated cli commands generated by protoc-gen-gocli
func Commands(clientFn func() {{.ServiceName}}) []cli.Command {
	return []cli.Command{ {{.Commands}}
	}
}
`)
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(buf, struct {
		GeneratorOptions
		ServiceName string
		Commands    string
	}{GeneratorOptions: options, ServiceName: serviceName, Commands: strings.Join(commands, "")})
	if err != nil {
		return "", err
	}
	return string(buf.Bytes()), nil
}
