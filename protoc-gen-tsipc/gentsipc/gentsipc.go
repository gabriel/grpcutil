package gentsrpc

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/descriptor"
)

func removePackage(s string) string {
	sp := strings.Split(s, ".")
	return sp[len(sp)-1]
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

func (cfg GeneratorOptions) streamMethodToIPC(name string, m *descriptor.Method) (string, string, []string) {
	serviceName := m.Service.GetName()
	methodName := *m.Name
	rpcName := lowerPrefix(methodName)
	requestType := removePackage(*m.InputType)
	responseType := removePackage(*m.OutputType)
	resultType := methodName + `Event`
	fullMethod := serviceName + "." + methodName
	types := []string{requestType, responseType}

	res := `export type ` + resultType + ` = {err?: {message: string; name: string; code?: number}, res?: ` + responseType + `, done: boolean}`

	s := `export const ` + rpcName + ` = (
  f: (res: ` + resultType + `) => void
): ((req?: ` + requestType + `, end?: boolean) => void) => {
  const reply = '` + fullMethod + `-' + replyID()
  ipcRenderer.on(reply, (event, arg) => {
    if (!!arg.done || arg.err) {
      ipcRenderer.removeAllListeners(reply)
	}
	if (arg.err) {
	  console.error('RPC-stream error (` + fullMethod + `):', arg.err)
      errHandler(arg.err)
    }
    if (!!arg.done) {
      console.log('` + fullMethod + `')
	}
	const res: ` + resultType + ` = {		
		res: arg.resp,
		err: arg.err,
		done: !!arg.done, 
	}
	f(res)
  })
  return (req?: ` + requestType + `, end?: boolean) => {
    ipcRenderer.send('rpc-stream', {service: '` + serviceName + `', method: '` + methodName + `', args: req, reply: reply, end: end})
    if (end) {
      ipcRenderer.removeAllListeners(reply)
    }
  }
}
`
	return strings.Join([]string{res, s}, "\n\n"), methodName, types
}

func (cfg GeneratorOptions) methodToIPC(name string, m *descriptor.Method) (string, string, []string) {
	serviceName := m.Service.GetName()
	methodName := *m.Name
	rpcName := lowerPrefix(methodName)
	requestType := removePackage(*m.InputType)
	responseType := removePackage(*m.OutputType)
	types := []string{requestType, responseType}
	fullMethod := serviceName + "." + methodName

	s := `export const ` + rpcName + ` = (req: ` + requestType + `) => {
	return new Promise<` + responseType + `>((resolve, reject) => {
		const reply = '` + fullMethod + `-' + replyID()
		ipcRenderer.on(reply, (event, arg) => {
			ipcRenderer.removeAllListeners(reply)
			if (arg.err) {
				console.error('RPC error (` + fullMethod + `):', arg.err)
				reject(arg.err)
				errHandler(arg.err)
			} else {
				console.log('` + fullMethod + `')
			}
			resolve(arg.resp)
		})
		ipcRenderer.send('rpc', {service: '` + serviceName + `', method: '` + methodName + `', args: req, reply: reply})
	})
}
`
	return s, methodName, types
}

func (cfg GeneratorOptions) serviceToRPC(s *descriptor.Service, reg *descriptor.Registry) (string, string, error) {
	types := []string{}
	result := []string{}
	methods := []string{}
	for _, m := range s.Methods {
		var ipc string
		var method string
		var typs []string
		if m.ClientStreaming != nil && *m.ClientStreaming && m.ServerStreaming != nil && *m.ServerStreaming {
			ipc, method, typs = cfg.streamMethodToIPC(*s.Name, m)
		} else {
			ipc, method, typs = cfg.methodToIPC(*s.Name, m)
		}

		result = append(result, ipc)
		methods = append(methods, method)
		types = append(types, typs...)
	}
	types = unique(types)
	return strings.Join(result, "\n"), strings.Join(types, ",\n  "), nil
}

func unique(strs []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range strs {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func generate(file *descriptor.File, registry *descriptor.Registry, options GeneratorOptions) (string, error) {
	out := []string{}
	types := []string{}
	f, err := registry.LookupFile(file.GetName())
	if err != nil {
		return "", err
	}

	name := file.GetName()
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	typesImport := "./" + base + ".d"

	for _, service := range f.Services {
		s, t, serr := options.serviceToRPC(service, registry)
		if serr != nil {
			return "", serr
		}
		out = append(out, s)
		types = append(types, t)
	}

	buf := new(bytes.Buffer)
	tmpl, err := template.New("").Parse(`// Code generated by protoc-gen-tsrpc DO NOT EDIT.
// InputID: {{.InputID}}
import {ipcRenderer} from 'electron'
import {randomBytes} from 'crypto'

import {{"{"}}
  {{.Types}}
{{"}"}} from '{{.TypesImport}}'

const replyID = (): string => {
  return randomBytes(20).toString('hex')
}

export type ErrHandler = (err: {message: string; code: number}) => void
var errHandler: ErrHandler = (err: {message: string; code: number}) => {}
export const setErrHandler = (eh: ErrHandler) => {
  errHandler = eh
}

{{.Out}}
`)
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(buf, struct {
		GeneratorOptions
		Types       string
		TypesImport string
		Out         string
	}{
		GeneratorOptions: options,
		Types:            strings.Join(types, ", "),
		TypesImport:      typesImport,
		Out:              strings.Join(out, "\n\n"),
	})
	if err != nil {
		return "", err
	}
	return string(buf.Bytes()), nil
}
