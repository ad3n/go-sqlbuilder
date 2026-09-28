package sqlbuilder

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"sync"
	"testing"
)

type fuzzState struct {
	data                    []byte
	dataIndex               int
	callchainRepresentation string
	currentBuilder          reflect.Value
	usedMethods             map[string]bool
}

func (fs *fuzzState) consumeData(size int) []byte {
	if len(fs.data) <= fs.dataIndex+size {
		return []byte{}
	}
	result := make([]byte, size)
	copy(result, fs.data[fs.dataIndex:fs.dataIndex+size])
	fs.dataIndex += size
	return result
}

func (fs *fuzzState) updateCallchain(method string, args []reflect.Value) {
	fs.callchainRepresentation += "." + method + "("
	for i, arg := range args {
		if i > 0 {
			fs.callchainRepresentation += ", "
		}
		fs.callchainRepresentation += fmt.Sprintf("%q", arg)
	}
	fs.callchainRepresentation += ")"
}

func getSelectBuilderMethods() (map[string]reflect.Type, []string) {
	sbType := reflect.TypeFor[*SelectBuilder]()

	skipMethods := []string{
		"Build", "String", "BuildWithFlavor", "Flavor",
		"NumCol", "NumValue", "NumAssignment", "TableNames", "Var",
	}

	methodList := make(map[string]reflect.Type)
	methodNames := make([]string, 0, sbType.NumMethod())

	for method := range sbType.Methods() {
		if slices.Contains(skipMethods, method.Name) {
			continue
		}

		methodList[method.Name] = method.Type
		methodNames = append(methodNames, method.Name)
	}

	return methodList, methodNames
}

func generateMethodArgs(methodType reflect.Type, state *fuzzState) ([]reflect.Value, bool) {
	numArgs := methodType.NumIn() - 1 // Skip receiver
	isVariadic := methodType.IsVariadic()

	if isVariadic {
		return generateVariadicArgs(methodType, numArgs, state)
	}
	return generateFixedArgs(methodType, numArgs, state)
}

func generateFixedArgs(methodType reflect.Type, numArgs int, state *fuzzState) ([]reflect.Value, bool) {
	args := make([]reflect.Value, numArgs)
	for i := range numArgs {
		argType := methodType.In(i + 1)
		argData := state.consumeData(16)
		args[i] = generateArgumentForType(argType, argData)

		if !args[i].IsValid() {
			return nil, false
		}

		if argType.Kind() == reflect.Pointer && args[i].Kind() == reflect.Pointer {
			if argType != args[i].Type() {
				return nil, false
			}
		}
	}

	return args, true
}

func generateVariadicArgs(methodType reflect.Type, numArgs int, state *fuzzState) ([]reflect.Value, bool) {
	numFixedArgs := numArgs - 1

	args := make([]reflect.Value, numFixedArgs)
	for i := range numFixedArgs {
		argType := methodType.In(i + 1)
		argData := state.consumeData(16)
		args[i] = generateArgumentForType(argType, argData)
		if !args[i].IsValid() {
			return nil, false
		}
	}

	if numFixedArgs < numArgs {
		variadicType := methodType.In(numArgs).Elem()
		numVariadicArgs := 0
		if len(state.data) > state.dataIndex {

			numVariadicArgs = int(state.data[state.dataIndex] % 4)
			state.dataIndex++
		}

		for j := 0; j < numVariadicArgs; j++ {
			argData := state.consumeData(16)
			varArg := generateArgumentForType(variadicType, argData)
			if !varArg.IsValid() {
				return nil, false
			}

			args = append(args, varArg)
		}
	}

	return args, true
}

func tryCallMethod(methodName string, methodType reflect.Type, state *fuzzState, t *testing.T) bool {

	callableMethod := state.currentBuilder.MethodByName(methodName)
	if !callableMethod.IsValid() {
		return false
	}

	args, canCall := generateMethodArgs(methodType, state)
	if !canCall {
		return false
	}

	state.updateCallchain(methodName, args)
	t.Log("callchain:", state.callchainRepresentation)

	state.usedMethods[methodName] = true

	result := callableMethod.Call(args)

	if len(result) > 0 && result[0].IsValid() {
		resultType := result[0].Type()
		if resultType.Kind() == reflect.Pointer &&
			resultType.String() == "*sqlbuilder.SelectBuilder" &&
			!result[0].IsNil() {
			state.currentBuilder = result[0]
		}
	}

	return true
}

func executeMethodChain(methodList map[string]reflect.Type, methodNames []string, state *fuzzState, maxChains uint8, t *testing.T) {
	for range maxChains {
		methodCalled := false

		for _, method := range methodNames {

			if state.usedMethods[method] && len(state.usedMethods) < len(methodNames) {
				continue
			}

			if tryCallMethod(method, methodList[method], state, t) {
				methodCalled = true
				break
			}
		}

		if !methodCalled {
			break
		}
	}
}

func finalizeBuild(state *fuzzState) {
	// Always try to build the final SQL to ensure it doesn't panic
	if state.currentBuilder.IsValid() {
		buildMethod := state.currentBuilder.MethodByName("Build")
		if buildMethod.IsValid() {
			buildMethod.Call([]reflect.Value{})
		}
	}
}

func FuzzSelect(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte, seed int64, numberOfChainedFunction uint8) {
		if len(data) == 0 {
			return
		}

		methodList, methodNames := getSelectBuilderMethods()

		r := rand.New(rand.NewSource(seed))
		r.Shuffle(len(methodNames), func(i, j int) {
			methodNames[i], methodNames[j] = methodNames[j], methodNames[i]
		})

		state := &fuzzState{
			data:                    data,
			dataIndex:               0,
			callchainRepresentation: "NewSelectBuilder()",
			currentBuilder:          reflect.ValueOf(NewSelectBuilder()),
			usedMethods:             make(map[string]bool),
		}

		maxChains := min(numberOfChainedFunction, 10)

		executeMethodChain(methodList, methodNames, state, maxChains, t)

		t.Logf("Final callchain: %s", state.callchainRepresentation)

		finalizeBuild(state)
	})
}

func generateArgumentForType(argType reflect.Type, data []byte) reflect.Value {
	switch argType.Kind() {
	case reflect.String:

		if argType.String() == "sqlbuilder.JoinOption" {
			joinOptions := []JoinOption{
				FullJoin, FullOuterJoin, InnerJoin,
				LeftJoin, LeftOuterJoin, RightJoin, RightOuterJoin,
			}
			if len(data) > 0 {
				return reflect.ValueOf(joinOptions[int(data[0])%len(joinOptions)])
			}

			return reflect.ValueOf(InnerJoin)
		}

		return reflect.ValueOf(string(data))
	case reflect.Int:

		if argType.String() == "sqlbuilder.Flavor" {
			return reflect.ValueOf(DefaultFlavor)
		}
		if len(data) > 0 {
			return reflect.ValueOf(int(data[0]))
		}
		return reflect.ValueOf(0)
	case reflect.Bool:
		if len(data) > 0 {
			return reflect.ValueOf(data[0]%2 == 0)
		}
		return reflect.ValueOf(false)
	case reflect.Int8:
		if len(data) > 0 {
			return reflect.ValueOf(int8(data[0]))
		}
		return reflect.ValueOf(int8(0))
	case reflect.Int16:
		if len(data) >= 2 {
			return reflect.ValueOf(int16(data[0])<<8 | int16(data[1]))
		}
		return reflect.ValueOf(int16(0))
	case reflect.Int32:
		if len(data) >= 4 {
			return reflect.ValueOf(int32(data[0])<<24 | int32(data[1])<<16 | int32(data[2])<<8 | int32(data[3]))
		}
		return reflect.ValueOf(int32(0))
	case reflect.Int64:
		if len(data) >= 8 {
			val := int64(data[0])<<56 | int64(data[1])<<48 | int64(data[2])<<40 | int64(data[3])<<32 |
				int64(data[4])<<24 | int64(data[5])<<16 | int64(data[6])<<8 | int64(data[7])
			return reflect.ValueOf(val)
		}
		return reflect.ValueOf(int64(0))
	case reflect.Uint:
		if len(data) > 0 {
			return reflect.ValueOf(uint(data[0]))
		}
		return reflect.ValueOf(uint(0))
	case reflect.Uint8:
		if len(data) > 0 {
			return reflect.ValueOf(uint8(data[0]))
		}
		return reflect.ValueOf(uint8(0))
	case reflect.Uint16:
		if len(data) >= 2 {
			return reflect.ValueOf(uint16(data[0])<<8 | uint16(data[1]))
		}
		return reflect.ValueOf(uint16(0))
	case reflect.Uint32:
		if len(data) >= 4 {
			return reflect.ValueOf(uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]))
		}
		return reflect.ValueOf(uint32(0))
	case reflect.Uint64:
		if len(data) >= 8 {
			val := uint64(data[0])<<56 | uint64(data[1])<<48 | uint64(data[2])<<40 | uint64(data[3])<<32 |
				uint64(data[4])<<24 | uint64(data[5])<<16 | uint64(data[6])<<8 | uint64(data[7])
			return reflect.ValueOf(val)
		}
		return reflect.ValueOf(uint64(0))
	case reflect.Float32:
		if len(data) >= 4 {
			bits := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
			return reflect.ValueOf(float32(bits))
		}
		return reflect.ValueOf(float32(0))
	case reflect.Float64:
		if len(data) >= 8 {
			bits := uint64(data[0])<<56 | uint64(data[1])<<48 | uint64(data[2])<<40 | uint64(data[3])<<32 |
				uint64(data[4])<<24 | uint64(data[5])<<16 | uint64(data[6])<<8 | uint64(data[7])
			return reflect.ValueOf(float64(bits))
		}
		return reflect.ValueOf(float64(0))
	case reflect.Slice:
		if argType.Elem().Kind() == reflect.String {
			return reflect.ValueOf([]string{string(data)})
		}
		if argType.Elem().Kind() == reflect.Interface {
			return reflect.ValueOf([]any{string(data)})
		}
		return reflect.ValueOf([]any{string(data)})
	case reflect.Pointer:

		if argType == reflect.TypeFor[*WhereClause]() {
			return reflect.ValueOf(NewWhereClause())
		}
		if argType == reflect.TypeFor[*SelectBuilder]() {
			return reflect.ValueOf(NewSelectBuilder())
		}
		if argType == reflect.TypeFor[*Args]() {
			return reflect.ValueOf(&Args{})
		}
		if argType == reflect.TypeFor[*CTEBuilder]() {
			return reflect.ValueOf(DefaultFlavor.NewCTEBuilder())
		}
		if argType == reflect.TypeFor[*InsertBuilder]() {
			return reflect.ValueOf(DefaultFlavor.NewInsertBuilder())
		}
		if argType == reflect.TypeFor[*UpdateBuilder]() {
			return reflect.ValueOf(DefaultFlavor.NewUpdateBuilder())
		}
		if argType == reflect.TypeFor[*DeleteBuilder]() {
			return reflect.ValueOf(DefaultFlavor.NewDeleteBuilder())
		}

		str := string(data)
		return reflect.ValueOf(&str)
	case reflect.Interface:

		if argType.String() == "sqlbuilder.Builder" {

			return reflect.ValueOf(NewSelectBuilder())
		}
		return reflect.ValueOf(string(data))
	default:

		return reflect.Zero(argType)
	}
}

func FuzzSelectClone(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte, seed int64, numberOfChainedFunction uint8) {
		if len(data) == 0 {
			return
		}

		methodList, methodNames := getSelectBuilderMethods()

		r := rand.New(rand.NewSource(seed))
		r.Shuffle(len(methodNames), func(i, j int) {
			methodNames[i], methodNames[j] = methodNames[j], methodNames[i]
		})

		base := NewSelectBuilder()
		baseState := &fuzzState{
			data:                    data,
			dataIndex:               0,
			callchainRepresentation: "NewSelectBuilder()",
			currentBuilder:          reflect.ValueOf(base),
			usedMethods:             make(map[string]bool),
		}

		maxChains := min(numberOfChainedFunction, 10)
		executeMethodChain(methodList, methodNames, baseState, maxChains, t)

		baseSQLBefore, baseArgsBefore := base.Build()

		cloneCount := int(r.Uint32()%4) + 1
		var wg sync.WaitGroup
		wg.Add(cloneCount)
		start := make(chan struct{})

		type result struct {
			sql  string
			args []any
		}
		results := make(chan result, cloneCount)

		for i := range cloneCount {

			offset := 0
			if len(data) > 0 {
				offset = (i * 17) % len(data)
			}

			go func(off int) {
				defer wg.Done()

				<-start

				c := base.Clone()
				st := &fuzzState{
					data:                    data,
					dataIndex:               off,
					callchainRepresentation: "Clone()",
					currentBuilder:          reflect.ValueOf(c),
					usedMethods:             make(map[string]bool),
				}
				executeMethodChain(methodList, methodNames, st, maxChains, t)
				finalizeBuild(st)
				s, a := c.Build()
				results <- result{sql: s, args: a}
			}(offset)
		}

		close(start)
		wg.Wait()
		close(results)

		baseSQLAfter, baseArgsAfter := base.Build()
		if baseSQLBefore != baseSQLAfter || !reflect.DeepEqual(baseArgsBefore, baseArgsAfter) {
			t.Fatalf("base builder mutated by clones:\n before: %s %v\n after: %s %v", baseSQLBefore, baseArgsBefore, baseSQLAfter, baseArgsAfter)
		}

		cloneA := base.Clone()
		sA1, aA1 := cloneA.Build()

		done := make(chan struct{})
		go func() {
			defer close(done)

			c2 := base.Clone()

			c2.OrderBy("id").Desc().Limit(1).Offset(0)
			_, _ = c2.Build()
		}()

		sA2, aA2 := cloneA.Build()
		if sA1 != sA2 || !reflect.DeepEqual(aA1, aA2) {
			t.Fatalf("cloneA changed after mutating another clone")
		}

		<-done

		cloneA.Limit(3).Asc()
		_ = cloneA.String()
		baseSQLFinal, baseArgsFinal := base.Build()
		if baseSQLFinal != baseSQLAfter || !reflect.DeepEqual(baseArgsFinal, baseArgsAfter) {
			t.Fatalf("base changed after modifying a clone")
		}

		for range results {
		}
	})
}
