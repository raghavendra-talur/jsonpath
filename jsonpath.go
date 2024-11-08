package jsonpath

import (
	"fmt"
	"reflect"
	"strings"

	"k8s.io/client-go/util/jsonpath"
)

func EvaluateCheckHook(jsonData interface{}, booleanExpression string) (bool, error) {
	op, jsonPaths, err := parseBooleanExpression(booleanExpression)
	if err != nil {
		return false, fmt.Errorf("failed to parse boolean expression: %w", err)
	}

	operand := make([]reflect.Value, 2)
	for i, jsonPath := range jsonPaths {
		operand[i], err = QueryJsonPath(jsonData, jsonPath)
		if err != nil {
			return false, fmt.Errorf("failed to get value for %v: %w", jsonPath, err)
		}
	}

	return compare(operand[0], operand[1], op)
}

func isKindString(kind reflect.Kind) bool {
	return kind == reflect.String
}

func isKindFloat64(kind reflect.Kind) bool {
	return kind == reflect.Float64
}

func isKindBool(kind reflect.Kind) bool {
	return kind == reflect.Bool
}

func isUnsupportedKind(kind reflect.Kind) bool {
	return kind != reflect.String && kind != reflect.Float64 && kind != reflect.Bool
}

func isKindStringOrFloat64(kind reflect.Kind) bool {
	return isKindString(kind) || isKindFloat64(kind)
}

// compare compares two interfaces using the specified operator
func compare(a, b reflect.Value, operator string) (bool, error) {
	// If one is int and another is float64
	// then convert int to float64 for comparison
	if a.Kind() == reflect.Int && b.Kind() == reflect.Float64 {
		a = reflect.ValueOf(float64(a.Int()))
	}
	if a.Kind() == reflect.Float64 && b.Kind() == reflect.Int {
		b = reflect.ValueOf(float64(b.Int()))
	}

	// if they are just an interface then we convert them to string
	if a.Kind() == reflect.Interface {
		a = reflect.ValueOf(fmt.Sprintf("%v", a.Interface()))
	}
	if b.Kind() == reflect.Interface {
		b = reflect.ValueOf(fmt.Sprintf("%v", b.Interface()))
	}

	if a.Kind() != b.Kind() {
		return false, fmt.Errorf("operands of different kinds can't be compared %v, %v", a.Kind(), b.Kind())
	}

	// At this point, both a and b should be of the same kind
	if operator != "==" && operator != "!=" {
		if !isKindStringOrFloat64(a.Kind()) || !isKindStringOrFloat64(b.Kind()) {
			return false, fmt.Errorf("operands not supported for operator: %v, %v, %s",
				a.Kind(), b.Kind(), operator)
		}
	}

	// Here, we have two scenarios:
	// 1. operands are either string or float64 and operator is any of the 6
	// 2. operands are of any kind and operator is either == or !=

	// Safety latch:
	// We will initially support only string, float64 and bool
	// return an error if we encounter any other kind
	if isUnsupportedKind(a.Kind()) || isUnsupportedKind(b.Kind()) {
		return false, fmt.Errorf("unsupported kind for comparison: %v, %v", a.Kind(), b.Kind())
	}

	if isKindBool(a.Kind()) && isKindBool(b.Kind()) {
		return compareBool(a.Bool(), b.Bool(), operator)
	}

	return compareValues(a.Interface(), b.Interface(), operator)
}

func compareBool(a, b bool, operator string) (bool, error) {
	switch operator {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}

func compareString(a, b, operator string) (bool, error) {
	switch operator {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	case ">=":
		return a >= b, nil
	case ">":
		return a > b, nil
	case "<=":
		return a <= b, nil
	case "<":
		return a < b, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}

func compareFloat64(a, b float64, operator string) (bool, error) {
	switch operator {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	case ">=":
		return a >= b, nil
	case ">":
		return a > b, nil
	case "<=":
		return a <= b, nil
	case "<":
		return a < b, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}

func compareValues(val1, val2 interface{}, operator string) (bool, error) {
	switch v1 := val1.(type) {
	case float64:
		v2, ok := val2.(float64)
		if !ok {
			return false, fmt.Errorf("mismatched types")
		}
		switch operator {
		case "==":
			return v1 == v2, nil
		case "!=":
			return v1 != v2, nil
		case "<":
			return v1 < v2, nil
		case ">":
			return v1 > v2, nil
		case "<=":
			return v1 <= v2, nil
		case ">=":
			return v1 >= v2, nil
		}
	case string:
		v2, ok := val2.(string)
		if !ok {
			return false, fmt.Errorf("mismatched types")
		}
		switch operator {
		case "==":
			return v1 == v2, nil
		case "!=":
			return v1 != v2, nil
		}
	case bool:
		v2, ok := val2.(bool)
		if !ok {
			return false, fmt.Errorf("mismatched types")
		}
		switch operator {
		case "==":
			return v1 == v2, nil
		case "!=":
			return v1 != v2, nil
		}
	}
	return false, fmt.Errorf("unsupported type or operator")
}

func isValidJsonPathExpression(expr string) bool {
	jp := jsonpath.New("validator").AllowMissingKeys(true)

	err := jp.Parse(expr)
	if err != nil {
		return false
	}

	_, err = QueryJsonPath("{}", expr)
	if err != nil {
		return false
	}

	return true
}

func QueryJsonPath(data interface{}, jsonPath string) (reflect.Value, error) {
	jp := jsonpath.New("extractor").AllowMissingKeys(true)

	if err := jp.Parse(jsonPath); err != nil {
		return reflect.Value{}, fmt.Errorf("failed to get value invalid jsonpath %w", err)
	}

	results, err := jp.FindResults(data)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("failed to get value from data using jsonpath %w", err)
	}

	if len(results) == 0 || len(results[0]) == 0 {
		return reflect.Value{}, nil
	}

	return results[0][0], nil
}

func trimLeadingTrailingWhiteSpace(paths []string) []string {
	tPaths := make([]string, len(paths))
	for i, path := range paths {
		tpath := strings.TrimSpace(path)
		tPaths[i] = tpath
	}
	return tPaths
}

func parseBooleanExpression(booleanExpression string) (op string, jsonPaths []string, err error) {
	operators := []string{"==", "!=", ">=", ">", "<=", "<"}

	// TODO
	// If one of the jsonpaths have the operator that we are looking for,
	// then strings.Split with split it at that point and not in the middle.

	for _, op := range operators {
		exprs := strings.Split(booleanExpression, op)

		jsonPaths = trimLeadingTrailingWhiteSpace(exprs)

		if len(exprs) == 2 &&
			isValidJsonPathExpression(jsonPaths[0]) &&
			isValidJsonPathExpression(jsonPaths[1]) {

			return op, jsonPaths, nil
		}
	}

	return "", []string{}, fmt.Errorf("unable to parse boolean expression %v", booleanExpression)
}
