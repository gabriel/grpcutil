package genjsrpc

import (
	"bytes"
	"strings"
	"text/template"
	"unicode"

	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/descriptor"
	strcase "github.com/stoewer/go-strcase"
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

func (cfg GeneratorOptions) methodToRedux(name string, m *descriptor.Method) (string, string, []string) {
	methodName := lowerPrefix(*m.Name)
	requestType := removePackage(*m.InputType)
	responseType := removePackage(*m.OutputType)
	types := []string{requestType, responseType}
	prefix := strings.ToUpper(name)
	actionName := prefix + "_" + strings.ToUpper(strcase.SnakeCase(methodName))
	s := `export const ` + methodName + ` = (req: ` + requestType + `, respFn: ?(resp: ` + responseType + `) => void, errFn: ?(err: RPCError) => void) => async (dispatch: (action: any) => void) => {
  dispatch({
    type: '` + actionName + `_REQUEST',
		payload: req,
	})
	let cl = await client()
  cl.` + methodName + `(req, (err: ?RPCError, resp: ?` + responseType + `) => {
    if (err && !errFn) {
      dispatch({
        type: 'ERROR',
        payload: {error: err, action: '` + actionName + `', req},
      })
      return
    }
		if (err && errFn) {
			console.error(err)
			errFn(err)
		}
		if (resp && respFn) respFn(resp)
    dispatch({
      type: '` + actionName + `_RESPONSE',
      payload: resp,
    })
  })
}`
	return s, methodName, types
}

func (cfg GeneratorOptions) methodToReducerActions(name string, m *descriptor.Method) (reducer string, reducerStates string, initialStates string, actionTypes string) {
	methodName := lowerPrefix(*m.Name)
	prefix := strings.ToUpper(name)
	actionName := prefix + "_" + strings.ToUpper(strcase.SnakeCase(methodName))
	requestType := removePackage(*m.InputType)
	responseType := removePackage(*m.OutputType)

	reducer = `
		case '` + actionName + `_REQUEST': {
			return {
				...state,
				` + methodName + `Loading: false,
				` + methodName + `Request: action.payload,
		  }
		}
		case '` + actionName + `_RESPONSE': {
			return {
				...state,
				` + methodName + `Loading: false,
				` + methodName + `Request: null,
				` + methodName + `: action.payload,
			}
		}`

	reducerStates = strings.Join([]string{
		methodName + `Loading: boolean`,
		methodName + `Request: ?` + requestType,
		methodName + `: ?` + responseType,
	}, ",\n  ")

	initialStates = strings.Join([]string{
		methodName + `Loading: false`,
		methodName + `Request: null`,
		methodName + `: null`,
	}, ",\n  ")

	actionTypes = methodName + `: (req: ` + requestType + `, respFn: ?(resp: ` + responseType + `) => void) => void`
	return
}

func (cfg GeneratorOptions) reducers(methods []string, reducerActions []string, actionStates []string, initialStates []string, actionTypes []string) string {
	// actions := `export type RPCActions = {
	// ` + strings.Join(actionTypes, ",\n  ") + "\n}\n\n"

	meth := `export const RPC = {
  ` + strings.Join(methods, ",\n  ") + "\n}\n\n"

	state := `export type RPCState = {
  ` + strings.Join(actionStates, ",\n  ") + "\n}\n\n"

	initial := `const initialState: RPCState = {
	` + strings.Join(initialStates, ",\n  ") + "\n}\n\n"

	reducer := `export function reducer(state: RPCState = initialState, action: any) {
  switch (action.type) {` + strings.Join(reducerActions, "\n    ") + `
		default:
			return state
	}
}`
	return meth + state + initial + reducer
}

func (cfg GeneratorOptions) serviceToRPC(s *descriptor.Service, reg *descriptor.Registry) (string, string, error) {
	types := []string{}
	result := []string{}
	methods := []string{}
	reducerActions := []string{}
	reducerStates := []string{}
	initialStates := []string{}
	actionTypes := []string{}
	for _, m := range s.Methods {
		redux, method, reduxTypes := cfg.methodToRedux(*s.Name, m)
		result = append(result, redux)
		methods = append(methods, method)
		types = append(types, reduxTypes...)

		reducerAction, reducerState, initialState, actionType := cfg.methodToReducerActions(*s.Name, m)
		reducerActions = append(reducerActions, reducerAction)
		reducerStates = append(reducerStates, reducerState)
		initialStates = append(initialStates, initialState)
		actionTypes = append(actionTypes, actionType)
	}

	reducer := cfg.reducers(methods, reducerActions, reducerStates, initialStates, actionTypes)
	types = unique(types)
	return strings.Join(result, "\n") + "\n\n" + reducer, strings.Join(types, ",\n  "), nil
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

func generateJSRPC(file *descriptor.File, registry *descriptor.Registry, options GeneratorOptions) (string, error) {
	redux := []string{}
	flowTypes := []string{}
	f, err := registry.LookupFile(file.GetName())
	if err != nil {
		return "", err
	}
	for _, service := range f.Services {
		s, t, serr := options.serviceToRPC(service, registry)
		if serr != nil {
			return "", serr
		}
		redux = append(redux, s)
		flowTypes = append(flowTypes, t)
	}

	buf := new(bytes.Buffer)
	tmpl, err := template.New("").Parse(`// @flow
/* eslint-disable */
// Code generated by protoc-gen-jsrpc DO NOT EDIT.
// InputID: {{.InputID}}
import {client} from './client'

import type {{"{"}}
  {{.FlowTypes}}
{{"}"}} from './types'

export type {{"{"}}
  {{.FlowTypes}}
{{"}"}}

export type RPCError = {
	code: number,
	message: string,
	details: string,
}

{{.Redux}}
`)
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(buf, struct {
		GeneratorOptions
		FlowTypes string
		Redux     string
		Reducers  string
	}{GeneratorOptions: options, FlowTypes: strings.Join(flowTypes, ", "), Redux: strings.Join(redux, "\n\n")})
	if err != nil {
		return "", err
	}
	return string(buf.Bytes()), nil
}
