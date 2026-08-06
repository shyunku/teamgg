package util

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

func InspectFunctionExecutionTime() func() {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		return nil
	}

	funcObj := runtime.FuncForPC(pc)
	if funcObj == nil {
		return nil
	}

	callers := make([]uintptr, 32)
	n := runtime.Callers(2, callers) // stack size
	frames := runtime.CallersFrames(callers[:n])
	// 현재 함수의 깊이를 찾습니다.
	depth := 0
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, "InspectFunctionExecutionTime") {
			break
		}
		if !more {
			break
		}
		depth++
	}
	if depth > 12 {
		depth -= 12
	}
	space := strings.Repeat("\t", depth)

	funcName := funcObj.Name()
	startTime := time.Now()

	fmt.Printf("[Debugger] %s --> <%s>\n", space, funcName)
	return func() {
		fmt.Printf("[Debugger] %s <-- <%s> : %v\n", space, funcName, time.Since(startTime))
	}
}
