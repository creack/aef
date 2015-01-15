// Package aef stands for Arguments, Environement, File
// Meaning the the commandline has the highest priority then environement
// then a file.
package aef

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

// Common errors
var (
	ErrFlagAlreadyParsed = fmt.Errorf("flag.Parse() needs to be classed by AEF, not dirrectly")
)

func resolveFlagName(fieldName string, t reflect.StructTag) string {
	if aefName := t.Get("aef"); aefName != "" {
		return aefName
	}
	if jsonName := t.Get("json"); jsonName != "" {
		return jsonName
	}
	return strings.ToLower(fieldName)
}

func loadCommandlineFlags(ss interface{}, setMap map[string]bool) error {
	// If commandline already parsed, error out.
	if flag.Parsed() {
		return ErrFlagAlreadyParsed
	}

	st := reflect.ValueOf(ss)
	s := st.Elem()
	typeOfT := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		ptr := unsafe.Pointer(f.Addr().Pointer())
		flagName := resolveFlagName(typeOfT.Field(i).Name, typeOfT.Field(i).Tag)
		usage := typeOfT.Field(i).Tag.Get("aef_usage")
		if usage == "" {
			usage = "Set " + flagName
		}
		switch f.Type().Kind() {
		case reflect.Bool:
			flag.BoolVar((*bool)(ptr), flagName, false, usage)
		case reflect.String:
			flag.StringVar((*string)(ptr), flagName, "", usage)
		case reflect.Int:
			flag.IntVar((*int)(ptr), flagName, 0, usage)
		case reflect.Int64:
			flag.Int64Var((*int64)(ptr), flagName, 0, usage)
		case reflect.Uint:
			flag.UintVar((*uint)(ptr), flagName, 0, usage)
		case reflect.Uint64:
			flag.Uint64Var((*uint64)(ptr), flagName, 0, usage)
		case reflect.Float64:
			flag.Float64Var((*float64)(ptr), flagName, 0, usage)
		default:
			if typeOfT.Field(i).Tag.Get("aef") != "-" {
				return fmt.Errorf("Unkown type: %s, use `aef:\"-\"` to force discard", f.Type())
			}
		}
	}
	// Parse command line
	flag.Parse()

	// Check which flags have been set
	flag.Visit(func(f *flag.Flag) {
		println("---------->", f.Name)
		setMap[f.Name] = true
	})
	return nil
}

func loadEnviron(ss interface{}, setMap map[string]bool) error {
	st := reflect.ValueOf(ss)
	s := st.Elem()
	typeOfT := s.Type()

	// If everything is set, stop here.
	if len(setMap) == s.NumField() {
		return nil
	}

	// Load missing fields from Environement
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		flagNameOrig := resolveFlagName(typeOfT.Field(i).Name, typeOfT.Field(i).Tag)
		flagName := strings.Replace(flagNameOrig, "-", "_", -1)

		// If already set, skip
		if setMap[flagNameOrig] {
			continue
		}

		// Mixedcase
		envVal := os.Getenv(flagName)
		if envVal == "" {
			// uppercase
			envVal = os.Getenv(strings.ToUpper(flagName))
		}
		if envVal == "" {
			// lowercase
			envVal = os.Getenv(strings.ToLower(flagName))
		}
		if envVal != "" {
			switch f.Type().Kind() {
			case reflect.Bool:
				v, _ := strconv.ParseBool(envVal)
				f.SetBool(v)
			case reflect.String:
				f.SetString(envVal)
			case reflect.Int:
				fallthrough
			case reflect.Int64:
				v, err := strconv.ParseInt(envVal, 10, 64)
				if err != nil {
					continue
				}
				f.SetInt(v)
			case reflect.Uint:
				fallthrough
			case reflect.Uint64:
				v, err := strconv.ParseUint(envVal, 10, 64)
				if err != nil {
					continue
				}
				f.SetUint(v)
			case reflect.Float64:
				v, err := strconv.ParseFloat(envVal, 64)
				if err != nil {
					continue
				}
				f.SetFloat(v)
			default:
				if typeOfT.Field(i).Tag.Get("aef") != "-" {
					return fmt.Errorf("Unkown type: %s, use `aef:\"-\"` to force discard", f.Type())
				}
			}
			setMap[flagNameOrig] = true
		}
	}
	return nil
}

var dumpFile = ioutil.ReadFile

func loadFile(ss interface{}, setMap map[string]bool, filePath string) error {
	st := reflect.ValueOf(ss)
	s := st.Elem()
	typeOfT := s.Type()

	// If no file given or everything is set, stop here.
	if filePath == "" || len(setMap) == s.NumField() {
		return nil
	}
	if len(filePath) > 0 && filePath[0] == '~' {
		filePath = os.Getenv("HOME") + filePath[1:]
	}
	f, err := dumpFile(filePath)
	if err != nil {
		// If error, discard and stop
		return nil
	}

	m := map[string]interface{}{}
	if err := json.Unmarshal(f, &m); err != nil {
		// If error, discard and stop
		return nil
	}
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		for _, flagName := range []string{
			typeOfT.Field(i).Tag.Get("aef"),
			typeOfT.Field(i).Tag.Get("json"),
			strings.ToLower(typeOfT.Field(i).Name),
		} {

			iface, exists := m[flagName]
			if !exists {
				continue
			}

			// If already set, skip
			if setMap[flagName] {
				continue
			}

			switch v := iface.(type) {
			case bool:
				f.SetBool(v)
			case int:
				f.SetInt(int64(v))
			case int64:
				f.SetInt(v)
			case uint:
				f.SetUint(uint64(v))
			case uint64:
				f.SetUint(v)
			case float64:
				f.SetFloat(v)
			case []byte:
				f.SetBytes(v)
			case string:
				f.SetString(v)
			default:
				if typeOfT.Field(i).Tag.Get("aef") != "-" {
					return fmt.Errorf("Unkown type: %T, use `aef:\"-\"` to force discard", v)
				}
			}
			setMap[flagName] = true
		}
	}
	return nil
}

// Load will introspect the given interface and populate it
// from the commandline, environ and given file.
func Load(ss interface{}, filePath string) error {
	setMap := map[string]bool{}
	if err := loadCommandlineFlags(ss, setMap); err != nil {
		return err
	}
	if err := loadEnviron(ss, setMap); err != nil {
		return err
	}
	if err := loadFile(ss, setMap, filePath); err != nil {
		return err
	}
	return nil
}
